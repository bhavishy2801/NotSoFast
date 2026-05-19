package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s: %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func fixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	src := t.TempDir()
	gitTest(t, src, "init")
	for p, b := range map[string]string{"config/database.yaml": "db: local", "src/main.go": "package main", ".hidden/x": "marker", "root.txt": "root"} {
		f := filepath.Join(src, p)
		os.MkdirAll(filepath.Dir(f), 0700)
		if e := os.WriteFile(f, []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	gitTest(t, src, "add", ".")
	gitTest(t, src, "commit", "-m", "fixture")
	head := gitTest(t, src, "rev-parse", "HEAD")
	cfg := Config{Root: t.TempDir(), Repositories: map[string]string{"demo": src}, Principals: map[string]Principal{"alice": {Repositories: []string{"demo"}, Write: true}}, Policies: map[string]Policy{"unique": {Version: "1", Kind: "exact_basename", DestinationPrefix: "", MaxBytes: 1024}}}
	s, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if _, e = s.Register(context.Background(), "alice", "demo", head); e != nil {
		t.Fatal(e)
	}
	return s, src, head
}
func TestIncompleteAndPublication(t *testing.T) {
	s, src, head := fixture(t)
	ctx := context.Background()
	p := Predicate{Kind: "exact_basename", Value: []byte("database.yaml"), Version: 1}
	r, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p, Scope: Scope{Prefixes: []string{"src"}}})
	if e != nil {
		t.Fatal(e)
	}
	claim := Claim{Repository: "demo", Snapshot: head, Predicate: p, Receipts: []string{r.ID}}
	d, e := s.Verify(ctx, "alice", claim)
	if e != nil || d.Outcome != "UNKNOWN" {
		t.Fatalf("%+v %v", d, e)
	}
	r, e = s.SearchMissing(ctx, "alice", claim)
	if e != nil {
		t.Fatal(e)
	}
	claim.Receipts = append(claim.Receipts, r.ID)
	d, e = s.Verify(ctx, "alice", claim)
	if e != nil || d.Outcome != "REFUTED" {
		t.Fatalf("%+v %v", d, e)
	}
	p.Value = []byte("fresh.yaml")
	r, e = s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p})
	if e != nil {
		t.Fatal(e)
	}
	req := CreateRequest{Repository: "demo", Snapshot: head, Operation: "one", Policy: "unique", PolicyVersion: "1", Path: "fresh.yaml", Content: []byte("hello"), Receipts: []string{r.ID}}
	op, e := s.GuardedCreate(ctx, "alice", req)
	if e != nil {
		t.Fatal(e)
	}
	if op.Base != head || op.Candidate == head {
		t.Fatal(op)
	}
	retry, e := s.GuardedCreate(ctx, "alice", req)
	if e != nil || retry.Candidate != op.Candidate {
		t.Fatalf("retry: %+v %v", retry, e)
	}
	req.Operation = "two"
	if _, e = s.GuardedCreate(ctx, "alice", req); Code(e) != "STATE_CHANGED" {
		t.Fatalf("stale: %v", e)
	}
	if gitTest(t, src, "rev-parse", "HEAD") != head || gitTest(t, src, "status", "--porcelain") != "" {
		t.Fatal("source modified")
	}
}
