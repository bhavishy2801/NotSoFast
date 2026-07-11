package core

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAblations(t *testing.T) {
	if os.Getenv("NSF_BENCH") == "" {
		t.Skip("explicit benchmark only")
	}
	ctx := context.Background()
	type result struct {
		Mode      string `json:"mode"`
		TotalNS   int64  `json:"total_ns"`
		Evaluated int    `json:"evaluated"`
		Reused    int    `json:"reused"`
		Outcome   string `json:"outcome"`
	}
	var results []result
	for _, mode := range []string{"no_cross_snapshot_reuse", "no_composition", "full"} {
		s, _, head := fixture(t)
		p := Predicate{"literal_bytes", []byte("new content"), 1}
		left := searchTest(t, s, head, p, Scope{Prefixes: []string{"src"}})
		right := searchTest(t, s, head, p, Scope{Paths: []string{"config/database.yaml", ".hidden/x", "root.txt"}})
		absent := searchTest(t, s, head, Predicate{"exact_basename", []byte("new.txt"), 1}, Scope{})
		op, e := s.GuardedCreate(ctx, "alice", CreateRequest{Repository: "demo", Snapshot: head, Operation: "ablation", Policy: "unique", PolicyVersion: "1", Path: "new.txt", Content: []byte("new content"), Receipts: []string{absent.ID}})
		if e != nil {
			t.Fatal(e)
		}
		start := time.Now()
		q := SearchRequest{Repository: "demo", Snapshot: op.Candidate, Predicate: p, Parents: []string{left.ID, right.ID}}
		if mode == "no_cross_snapshot_reuse" {
			q.Fresh = true
			q.Parents = nil
		}
		if mode == "no_composition" {
			q.Parents = nil
		}
		r, e := s.Search(ctx, "alice", q)
		if e != nil {
			t.Fatal(e)
		}
		d, e := s.Verify(ctx, "alice", Claim{Repository: "demo", Snapshot: op.Candidate, Predicate: p, Receipts: []string{r.ID}})
		if e != nil || d.Outcome != "REFUTED" {
			t.Fatal(d, e)
		}
		results = append(results, result{mode, time.Since(start).Nanoseconds(), r.Stats.Evaluated, r.Stats.Reused, d.Outcome})
	}
	b, _ := json.MarshalIndent(results, "", "  ")
	if e := os.WriteFile("../docs/ablation-results.json", b, 0600); e != nil {
		t.Fatal(e)
	}
}
