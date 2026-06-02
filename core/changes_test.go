package core

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRenamesDeletesTypesAndUnusualPaths(t *testing.T) {
	s, _, head := fixture(t)
	ctx := context.Background()
	dir := s.repo("demo")
	p := Predicate{"literal_bytes", []byte("marker"), 1}
	old := searchTest(t, s, head, p, Scope{})
	blob, e := gitText(ctx, dir, []byte("marker"), "hash-object", "-w", "--stdin")
	if e != nil {
		t.Fatal(e)
	}
	// Build several fully identified snapshots, including symlinks and external gitlinks.
	for _, rows := range []string{
		"100644 blob " + blob + "\trenamed\tfile\n.txt\x00",
		"120000 blob " + blob + "\tsymlink\x00",
		"160000 commit " + strings.Repeat("1", 40) + "\tsubmodule\x00",
		"100755 blob " + blob + "\téxécutable.txt\x00",
		"",
	} {
		tree, e := gitText(ctx, dir, []byte(rows), "mktree", "-z", "--missing")
		if e != nil {
			t.Fatal(e)
		}
		commit, e := gitText(ctx, dir, []byte("fixture mutation"), "commit-tree", tree, "-p", head)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = git(ctx, dir, nil, "update-ref", branch, commit, head); e != nil {
			t.Fatal(e)
		}
		r, e := s.Search(ctx, "alice", SearchRequest{Repository: "demo", Snapshot: commit, Predicate: p, Parents: []string{old.ID}})
		if e != nil {
			t.Fatal(e)
		}
		expected := oracleTest(t, dir, commit, p)
		got := map[string]string{}
		for _, v := range r.Evaluations {
			got[v.Entry.Path] = v.Result
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("%v != %v", got, expected)
		}
		d, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: commit, Predicate: p, Receipts: []string{r.ID}})
		if e != nil {
			t.Fatal(e)
		}
		want := "SUPPORTED"
		if bytes.Contains([]byte(rows), []byte("100644")) || bytes.Contains([]byte(rows), []byte("100755")) {
			want = "REFUTED"
		}
		if d.Outcome != want {
			t.Fatal(d)
		}
		head = commit
		old = r
	}
}
func TestAllEvidenceValidatedBeforeWitness(t *testing.T) {
	s, _, head := fixture(t)
	p := Predicate{"exact_basename", []byte("database.yaml"), 1}
	r := searchTest(t, s, head, p, Scope{})
	c := Claim{Repository: "demo", Snapshot: head, Predicate: p, Receipts: []string{r.ID, "forged"}}
	if d, e := s.Verify(context.Background(), "alice", c); d.Outcome != "INVALID_EVIDENCE" || Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(d, e)
	}
	c.Predicate.Value = []byte("other")
	c.Receipts = []string{r.ID}
	if _, e := s.Verify(context.Background(), "alice", c); Code(e) != "INVALID_EVIDENCE" {
		t.Fatal(e)
	}
}
