package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func searchTest(t *testing.T, s *Service, head string, p Predicate, scope Scope) Receipt {
	t.Helper()
	r, e := s.Search(context.Background(), "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p, Scope: scope})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestCompositionAndIntegrity(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	p := Predicate{Kind: "exact_basename", Value: []byte("missing"), Version: 1}
	a := searchTest(t, s, head, p, Scope{Prefixes: []string{"src"}})
	b := searchTest(t, s, head, p, Scope{Prefixes: []string{"config"}})
	c := Claim{Repository: "demo", Snapshot: head, Predicate: p, Receipts: []string{a.ID, a.ID, b.ID}}
	d, e := s.Verify(ctx, "alice", c)
	if e != nil || d.Outcome != "UNKNOWN" || d.Evaluated != 2 {
		t.Fatalf("overlap %+v %v", d, e)
	}
	r, e := s.Compose(ctx, "alice", c)
	if e != nil || r.Complete {
		t.Fatalf("composition %+v %v", r, e)
	}
	c.Receipts = []string{r.ID}
	full, e := s.SearchMissing(ctx, "alice", c)
	if e != nil {
		t.Fatal(e)
	}
	c.Receipts = []string{full.ID}
	d, e = s.Verify(ctx, "alice", c)
	if e != nil || d.Outcome != "SUPPORTED" {
		t.Fatalf("%+v %v", d, e)
	}
	c.Receipts = append(c.Receipts, "fabricated")
	d, e = s.Verify(ctx, "alice", c)
	if Code(e) != "INVALID_EVIDENCE" || d.Outcome != "INVALID_EVIDENCE" {
		t.Fatal(d, e)
	}
	if _, e = s.Receipt(ctx, "nobody", full.ID); Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(e)
	}
	body, _ := json.Marshal(full)
	body[10] ^= 1
	if _, e = s.db.Exec("UPDATE receipts SET body=? WHERE id=?", body, full.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Receipt(ctx, "alice", full.ID); Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(e)
	}
}
func TestScopesPredicatesAndCancellation(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		p     Predicate
		scope Scope
		want  string
	}{
		{Predicate{"exact_path", []byte("root.txt"), 1}, Scope{}, "REFUTED"},
		{Predicate{"exact_basename", []byte("x"), 1}, Scope{}, "REFUTED"},
		{Predicate{"exact_basename", []byte("database.yaml"), 1}, Scope{Prefixes: []string{"src"}}, "SUPPORTED"},
		{Predicate{"literal_bytes", []byte("marker"), 1}, Scope{}, "REFUTED"},
		{Predicate{"literal_bytes", []byte("not found"), 1}, Scope{}, "SUPPORTED"},
		{Predicate{"exact_basename", []byte("x"), 1}, Scope{Prefixes: []string{"sr"}}, "SUPPORTED"},
	} {
		r := searchTest(t, s, head, tc.p, Scope{})
		d, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: head, Predicate: tc.p, Scope: tc.scope, Receipts: []string{r.ID}})
		if e != nil || d.Outcome != tc.want {
			t.Fatalf("%+v: %+v %v", tc, d, e)
		}
	}
	if _, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: Predicate{"literal_bytes", nil, 1}}); Code(e) != "BAD_PREDICATE" {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := s.Search(cancelCtx, "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: Predicate{"literal_bytes", []byte("hi"), 1}}); e == nil {
		t.Fatal("cancel ignored")
	}
	s.cfg.MaxBlobBytes = 1
	r := searchTest(t, s, head, Predicate{"literal_bytes", []byte("oversize-query"), 1}, Scope{})
	if r.Complete {
		t.Fatal("oversize marked complete")
	}
	d, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: head, Predicate: r.Predicate, Receipts: []string{r.ID}})
	if e != nil || d.Outcome != "UNKNOWN" {
		t.Fatal(d, e)
	}
}
func TestRefreshAgainstIndependentOracle(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	p := Predicate{"literal_bytes", []byte("new marker"), 1}
	old := searchTest(t, s, head, p, Scope{})
	// An authorized create changes the domain. Content reuse must still evaluate the new object.
	absent := searchTest(t, s, head, Predicate{"exact_basename", []byte("new.txt"), 1}, Scope{})
	op, e := s.GuardedCreate(ctx, "alice", CreateRequest{Repository: "demo", Snapshot: head, Operation: "refresh", Policy: "unique", PolicyVersion: "1", Path: "new.txt", Content: []byte("new marker"), Receipts: []string{absent.ID}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: op.Candidate, Predicate: p, Parents: []string{old.ID}})
	if e != nil {
		t.Fatal(e)
	}
	if r.Stats.Evaluated != 1 || r.Stats.Reused != 4 {
		t.Fatal(r.Stats)
	}
	oracle := oracleTest(t, s.repo("demo"), op.Candidate, p)
	got := map[string]string{}
	for _, v := range r.Evaluations {
		got[v.Entry.Path] = v.Result
	}
	if !reflect.DeepEqual(got, oracle) {
		t.Fatalf("derived %v oracle %v", got, oracle)
	}
	if _, e = s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: op.Candidate, Predicate: p, Receipts: []string{old.ID}}); Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(e)
	}
}
func TestMissingObjectAndEviction(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	p := Predicate{"exact_basename", []byte("fresh"), 1}
	r := searchTest(t, s, head, p, Scope{})
	if _, e := s.db.Exec("DELETE FROM receipts WHERE id=?", r.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: head, Predicate: p, Receipts: []string{r.ID}}); Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(e)
	}
	// Corrupt a managed pack: enumeration must fail, never establish empty coverage.
	files, e := filepath.Glob(filepath.Join(s.repo("demo"), "objects", "pack", "*.pack"))
	if e != nil || len(files) != 1 {
		t.Fatal(files, e)
	}
	if e = os.Chmod(files[0], 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(files[0], []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p}); e == nil {
		t.Fatal("corrupt manifest accepted")
	}
}
func FuzzScopeBoundaries(f *testing.F) {
	for _, s := range []string{"src", "s", ".hidden", "x/y", "x\tq", "x\nq"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if !validPath(p) {
			return
		}
		scope := Scope{Prefixes: []string{p}}
		if !scope.selects(Entry{Path: p + "/child"}) || scope.selects(Entry{Path: p + "other/child"}) {
			t.Fatal("prefix boundary")
		}
	})
}
