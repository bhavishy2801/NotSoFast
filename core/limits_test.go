package core

import (
	"context"
	"testing"
)

func TestSingleInstanceAndStorageLimit(t *testing.T) {
	s, _, head := fixture(t)
	other, e := Open(s.cfg)
	if e == nil {
		other.Close()
		t.Fatal("second instance accepted")
	}
	s.cfg.MaxStorageBytes = 1
	if _, e = s.Search(context.Background(), "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: Predicate{"exact_basename", []byte("x"), 1}}); Code(e) != "STORAGE_LIMIT" {
		t.Fatal(e)
	}
}

func TestCandidateCannotExceedManifestLimit(t *testing.T) {
	s, _, head := fixture(t)
	s.cfg.MaxEntries = 7
	r := searchTest(t, s, head, Predicate{"exact_basename", []byte("new.txt"), 1}, Scope{})
	_, e := s.GuardedCreate(context.Background(), "alice", CreateRequest{Repository: "demo", Snapshot: head, Operation: "limit", Policy: "unique", PolicyVersion: "1", Path: "new.txt", Content: []byte("x"), Receipts: []string{r.ID}})
	if Code(e) != "LIMIT" {
		t.Fatal(e)
	}
	got, e := s.Head(context.Background(), "alice", "demo")
	if e != nil || got != head {
		t.Fatal(got, e)
	}
}
