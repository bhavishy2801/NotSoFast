package core

import (
	"context"
	"sync"
	"testing"
)

func TestConcurrentAndConflict(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	r := searchTest(t, s, head, Predicate{"exact_basename", []byte("unique.txt"), 1}, Scope{})
	q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "a", Policy: "unique", PolicyVersion: "1", Path: "unique.txt", Content: []byte("ok"), Receipts: []string{r.ID}}
	var wg sync.WaitGroup
	codes := make(chan string, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			req := q
			req.Operation = id
			_, e := s.GuardedCreate(ctx, "alice", req)
			codes <- Code(e)
		}(id)
	}
	wg.Wait()
	close(codes)
	seen := map[string]int{}
	for code := range codes {
		seen[code]++
	}
	if seen[""] != 1 || seen["STATE_CHANGED"] != 1 {
		t.Fatal(seen)
	}
	var winner string
	for _, id := range []string{"a", "b"} {
		if _, e := s.Operation(ctx, "alice", "demo", id); e == nil {
			winner = id
		}
	}
	q.Operation = winner
	q.Content = []byte("different")
	if _, e := s.GuardedCreate(ctx, "alice", q); Code(e) != "OPERATION_CONFLICT" {
		t.Fatal(e)
	}
	head2, e := s.Head(ctx, "alice", "demo")
	if e != nil {
		t.Fatal(e)
	}
	r2, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: head2, Predicate: r.Predicate, Parents: []string{r.ID}})
	if e != nil {
		t.Fatal(e)
	}
	d, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: head2, Predicate: r.Predicate, Receipts: []string{r2.ID}})
	if e != nil || d.Outcome != "REFUTED" {
		t.Fatal(d, e)
	}
}
func TestPoliciesAndRecovery(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	s.cfg.Policies["marker"] = Policy{Version: "1", Kind: "literal_bytes", Marker: []byte("owned: xyz"), MaxBytes: 100}
	if e := s.activatePolicy(ctx, "demo"); e != nil {
		t.Fatal(e)
	}
	r := searchTest(t, s, head, Predicate{"literal_bytes", []byte("owned: xyz"), 1}, Scope{})
	names := searchTest(t, s, head, Predicate{"exact_basename", []byte("new.txt"), 1}, Scope{})
	q := CreateRequest{Repository: "demo", Snapshot: head, Operation: "marker", Policy: "marker", PolicyVersion: "1", Path: "sub/new.txt", Content: []byte("owned: xyz"), Receipts: []string{r.ID, names.ID}}
	bad := q
	bad.PolicyVersion = "0"
	if _, e := s.GuardedCreate(ctx, "alice", bad); Code(e) != "POLICY_CHANGED" {
		t.Fatal(e)
	}
	bad = q
	bad.Content = []byte("no marker")
	if _, e := s.GuardedCreate(ctx, "alice", bad); Code(e) != "POLICY_DENIED" {
		t.Fatal(e)
	}
	for _, p := range []string{"../out", "/tmp/x", "a//b", "a/../b", ".git/config", "root.txt/child", "src/main.go"} {
		bad = q
		bad.Path = p
		if _, e := s.GuardedCreate(ctx, "alice", bad); e == nil {
			t.Fatal("accepted", p)
		}
	}
	op, e := s.GuardedCreate(ctx, "alice", q)
	if e != nil {
		t.Fatal(e)
	}
	cfg := s.cfg
	s.Close()
	reopened, e := Open(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	again, e := reopened.GuardedCreate(ctx, "alice", q)
	if e != nil || again.Candidate != op.Candidate {
		t.Fatal(again, e)
	}
}
