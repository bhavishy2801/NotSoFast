package core

import (
	"context"
	"testing"
)

func TestActivePolicyEpoch(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	r := searchTest(t, s, head, Predicate{"exact_basename", []byte("new.txt"), 1}, Scope{})
	other, e := gitText(ctx, s.repo("demo"), []byte("operator changed policy"), "hash-object", "-w", "--stdin")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = git(ctx, s.repo("demo"), nil, "update-ref", "refs/notsofast/policy", other); e != nil {
		t.Fatal(e)
	}
	_, e = s.GuardedCreate(ctx, "alice", CreateRequest{Repository: "demo", Snapshot: head, Operation: "stale-policy", Policy: "unique", PolicyVersion: "1", Path: "new.txt", Content: []byte("ok"), Receipts: []string{r.ID}})
	if Code(e) != "POLICY_CHANGED" {
		t.Fatal(e)
	}
}
