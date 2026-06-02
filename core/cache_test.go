package core

import (
	"context"
	"testing"
)

func TestCacheCorruptionCannotApprove(t *testing.T) {
	s, _, head := fixture(t)
	p := Predicate{"exact_basename", []byte("database.yaml"), 1}
	searchTest(t, s, head, p, Scope{})
	if _, e := s.db.Exec("UPDATE cache SET result='NO_MATCH' WHERE result='MATCH'"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Search(context.Background(), "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p}); e == nil {
		t.Fatal("corrupt cache became authoritative")
	}
}
