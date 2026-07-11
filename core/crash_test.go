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
	"time"
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

func TestTerminationDuringReferenceCommit(t *testing.T) {
	for _, delay := range []time.Duration{0, time.Millisecond, 20 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			s, _, head := fixture(t)
			ctx := context.Background()
			r := searchTest(t, s, head, Predicate{"exact_basename", []byte("kill.txt"), 1}, Scope{})
			q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "kill", Policy: "unique", PolicyVersion: "1", Path: "kill.txt", Content: []byte("ok"), Receipts: []string{r.ID}}
			op, e := s.GuardedCreate(ctx, "alice", q)
			if e != nil {
				t.Fatal(e)
			}
			dir := s.repo("demo")
			meta, e := gitText(ctx, dir, nil, "rev-parse", intentRef("alice", q.Operation))
			if e != nil {
				t.Fatal(e)
			}
			if _, e = git(ctx, dir, nil, "update-ref", branch, head, op.Candidate); e != nil {
				t.Fatal(e)
			}
			if _, e = git(ctx, dir, nil, "update-ref", "-d", operationRef("alice", q.Operation)); e != nil {
				t.Fatal(e)
			}
			cmd := exec.Command("git", "-c", "core.longpaths=true", "-c", "core.hooksPath="+os.DevNull, "-C", dir, "update-ref", "--stdin")
			in, _ := cmd.StdinPipe()
			out, _ := cmd.StdoutPipe()
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			defer cmd.Process.Kill()
			in.Write([]byte("start\nupdate " + branch + " " + op.Candidate + " " + head + "\ncreate " + operationRef("alice", q.Operation) + " " + meta + "\nprepare\n"))
			scan := bufio.NewScanner(out)
			prepared := false
			for scan.Scan() {
				if strings.Contains(scan.Text(), "prepare: ok") {
					prepared = true
					break
				}
			}
			if !prepared {
				t.Fatal("prepare failed")
			}
			if _, e = in.Write([]byte("commit\n")); e != nil {
				t.Fatal(e)
			}
			time.Sleep(delay)
			cmd.Process.Kill()
			cmd.Wait()
			actual, e := s.Head(ctx, "alice", "demo")
			if e != nil {
				t.Fatal(e)
			}
			got, e := s.Operation(ctx, "alice", "demo", q.Operation)
			if actual == op.Candidate {
				if e != nil || got.Candidate != op.Candidate {
					t.Fatal(got, e)
				}
			} else if actual == head {
				if Code(e) != "UNRESOLVED" {
					t.Fatal(got, e)
				}
			} else {
				t.Fatal("unexpected head", actual)
			}
			t.Logf("kill delay %s: head advanced=%t recovery=%s", delay, actual == op.Candidate, Code(e))
		})
	}
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
