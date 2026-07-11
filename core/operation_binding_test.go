package core

import (
	"context"
	"testing"
)

func TestCandidateIdentityBindsOperation(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	r := searchTest(t, s, head, Predicate{"exact_basename", []byte("same.txt"), 1}, Scope{})
	q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "first", Policy: "unique", PolicyVersion: "1", Path: "same.txt", Content: []byte("same"), Receipts: []string{r.ID}}
	first, e := s.GuardedCreate(ctx, "alice", q)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = git(ctx, s.repo("demo"), nil, "update-ref", branch, head, first.Candidate); e != nil {
		t.Fatal(e)
	}
	if _, e = git(ctx, s.repo("demo"), nil, "update-ref", "-d", operationRef("alice", q.Operation)); e != nil {
		t.Fatal(e)
	}
	q.Operation = "second"
	second, e := s.GuardedCreate(ctx, "alice", q)
	if e != nil {
		t.Fatal(e)
	}
	if first.Candidate == second.Candidate {
		t.Fatal("candidate identity aliases operations")
	}
	if _, e = s.Operation(ctx, "alice", "demo", "first"); Code(e) != "UNRESOLVED" {
		t.Fatal(e)
	}
}
