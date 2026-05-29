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
