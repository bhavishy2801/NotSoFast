package core

import (
	"context"
	"testing"
)

func TestEmptySHA256Manifest(t *testing.T) {
	src := t.TempDir()
	gitTest(t, src, "init", "--object-format=sha256")
	gitTest(t, src, "commit", "--allow-empty", "-m", "empty")
	head := gitTest(t, src, "rev-parse", "HEAD")
	s, e := Open(Config{Root: t.TempDir(), Repositories: map[string]string{"demo": src}, Principals: map[string]Principal{"alice": {Repositories: []string{"demo"}, Write: true}}, Policies: map[string]Policy{"unique": {Version: "1", Kind: "exact_basename", MaxBytes: 10}}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	snap, e := s.Register(context.Background(), "alice", "demo", head)
	if e != nil || snap.Algorithm != "sha256" || len(snap.Entries) != 0 {
		t.Fatal(snap, e)
	}
	d, e := s.Verify(context.Background(), "alice", Claim{Repository: "demo", Snapshot: head, Predicate: Predicate{"exact_basename", []byte("first"), 1}})
	if e != nil || d.Outcome != "SUPPORTED" {
		t.Fatal(d, e)
	}
	op, e := s.GuardedCreate(context.Background(), "alice", CreateRequest{Repository: "demo", Snapshot: head, Operation: "first", Policy: "unique", PolicyVersion: "1", Path: "first", Content: []byte("x")})
	if e != nil || len(op.Candidate) != 64 {
		t.Fatal(op, e)
	}
}
