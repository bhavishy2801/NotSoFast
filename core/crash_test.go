package core

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Subprocess stops before publication or immediately after it, without Close.
func TestCrashHelper(t *testing.T) {
	file := os.Getenv("NSF_CRASH_INPUT")
	if file == "" {
		return
	}
	var in struct {
		Config  Config
		Request CreateRequest
		Stage   string
	}
	b, e := os.ReadFile(file)
	if e != nil {
		os.Exit(91)
	}
	if json.Unmarshal(b, &in) != nil {
		os.Exit(92)
	}
	s, e := Open(in.Config)
	if e != nil {
		os.Exit(93)
	}
	if in.Stage == "before" {
		os.Exit(71)
	}
	if _, e = s.GuardedCreate(context.Background(), "alice", in.Request); e != nil {
		os.Exit(94)
	}
	os.Exit(72)
}
func TestProcessCrashRecovery(t *testing.T) {
	for _, stage := range []string{"before", "after"} {
		t.Run(stage, func(t *testing.T) {
			s, _, head := fixture(t)
			r := searchTest(t, s, head, Predicate{"exact_basename", []byte("crash.txt"), 1}, Scope{})
			q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "crash", Policy: "unique", PolicyVersion: "1", Path: "crash.txt", Content: []byte("ok"), Receipts: []string{r.ID}}
			cfg := s.cfg
			s.Close()
			b, _ := json.Marshal(struct {
				Config  Config
				Request CreateRequest
				Stage   string
			}{cfg, q, stage})
			file := filepath.Join(t.TempDir(), "input.json")
			os.WriteFile(file, b, 0600)
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), "NSF_CRASH_INPUT="+file)
			err := cmd.Run()
			exit, ok := err.(*exec.ExitError)
			if !ok || (stage == "before" && exit.ExitCode() != 71) || (stage == "after" && exit.ExitCode() != 72) {
				t.Fatal(err)
			}
			reopened, e := Open(cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			op, e := reopened.GuardedCreate(context.Background(), "alice", q)
			if e != nil {
				t.Fatal(e)
			}
			again, e := reopened.GuardedCreate(context.Background(), "alice", q)
			if e != nil || again.Candidate != op.Candidate {
				t.Fatal(again, e)
			}
			if n := gitTest(t, reopened.repo("demo"), "rev-list", "--count", head+".."+branch); n != "1" {
				t.Fatal("replayed", n)
			}
		})
	}
}
func TestGitPreparedTransactionTermination(t *testing.T) {
	s, _, head := fixture(t)
	dir := s.repo("demo")
	ctx := context.Background()
	tree, e := gitText(ctx, dir, nil, "rev-parse", head+"^{tree}")
	if e != nil {
		t.Fatal(e)
	}
	candidate, e := gitText(ctx, dir, []byte("candidate"), "commit-tree", tree, "-p", head)
	if e != nil {
		t.Fatal(e)
	}
	// Exercise the actual Git transaction protocol; terminate after prepare ACK.
	cmd := exec.Command("git", "-c", "core.hooksPath="+os.DevNull, "-C", dir, "update-ref", "--stdin")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	_, e = in.Write([]byte("start\nupdate " + branch + " " + candidate + " " + head + "\ncreate refs/notsofast/test-operation " + candidate + "\nprepare\n"))
	if e != nil {
		t.Fatal(e)
	}
	scan := bufio.NewScanner(out)
	prepared := false
	for scan.Scan() {
		if strings.Contains(scan.Text(), "prepare: ok") {
			prepared = true
			break
		}
	}
	if !prepared {
		t.Fatal("transaction not prepared")
	}
	cmd.Process.Kill()
	cmd.Wait()
	if got := gitTest(t, dir, "rev-parse", branch); got != head {
		t.Fatal("published before commit")
	}
	if _, e = git(ctx, dir, nil, "rev-parse", "--verify", "refs/notsofast/test-operation"); e == nil {
		t.Fatal("operation published before commit")
	}
	// SIGKILL can leave Git lockfiles: automatic removal is unsafe while a child may live.
	// Reads/recovery remain safe; operator repair is documented for write availability.
}
