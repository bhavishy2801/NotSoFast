package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportLaterSnapshotKeepsManagedHead(t *testing.T) {
	s, src, head := fixture(t)
	ctx := context.Background()
	p := Predicate{"literal_bytes", []byte("marker"), 1}
	old := searchTest(t, s, head, p, Scope{})
	if e := os.Rename(filepath.Join(src, "root.txt"), filepath.Join(src, "renamed.txt")); e != nil {
		t.Fatal(e)
	}
	gitTest(t, src, "add", "-A")
	gitTest(t, src, "commit", "-m", "rename")
	next := gitTest(t, src, "rev-parse", "HEAD")
	if _, e := s.Register(ctx, "alice", "demo", next); e != nil {
		t.Fatal(e)
	}
	h, e := s.Head(ctx, "alice", "demo")
	if e != nil || h != head {
		t.Fatal(h, e)
	}
	r, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: next, Predicate: p, Parents: []string{old.ID}})
	if e != nil {
		t.Fatal(e)
	}
	want := oracleTest(t, s.repo("demo"), next, p)
	for _, v := range r.Evaluations {
		if want[v.Entry.Path] != v.Result {
			t.Fatal(v)
		}
	}
}
