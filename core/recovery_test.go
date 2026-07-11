package core

import (
	"context"
	"testing"
)

func TestIntentRecoversMissingOperationReference(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	r := searchTest(t, s, head, Predicate{"exact_basename", []byte("recover.txt"), 1}, Scope{})
	q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "recover", Policy: "unique", PolicyVersion: "1", Path: "recover.txt", Content: []byte("ok"), Receipts: []string{r.ID}}
	op, e := s.GuardedCreate(ctx, "alice", q)
	if e != nil {
		t.Log(gitTest(t, s.repo("demo"), "show-ref"))
		t.Log(gitTest(t, s.repo("demo"), "show", intentRef("alice", q.Operation)+":operation.json"))
		t.Fatal(e)
	}
	// Reproduce both partial reference states deterministically, using real Git refs.
	if _, e = git(ctx, s.repo("demo"), nil, "update-ref", "-d", operationRef("alice", q.Operation)); e != nil {
		t.Fatal(e)
	}
	recovered, e := s.GuardedCreate(ctx, "alice", q)
	if e != nil || recovered.Candidate != op.Candidate {
		t.Fatal(recovered, e)
	}
	if _, e = git(ctx, s.repo("demo"), nil, "update-ref", branch, head, op.Candidate); e != nil {
		t.Fatal(e)
	}
	pending, e := s.Operation(ctx, "alice", "demo", q.Operation)
	if Code(e) != "UNRESOLVED" || pending.Digest != op.Digest {
		t.Fatal(pending, e)
	}
	if _, e = s.GuardedCreate(ctx, "alice", q); Code(e) != "UNRESOLVED" {
		t.Fatal(e)
	}
	q.Content = []byte("different")
	if _, e = s.GuardedCreate(ctx, "alice", q); Code(e) != "OPERATION_CONFLICT" {
		t.Fatal(e)
	}
}
