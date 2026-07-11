package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLargerFilenameWorkload(t *testing.T) {
	if os.Getenv("NSF_BENCH") == "" {
		t.Skip("explicit benchmark only")
	}
	src := t.TempDir()
	gitTest(t, src, "init")
	for i := 0; i < 4096; i++ {
		if e := os.WriteFile(filepath.Join(src, fmt.Sprintf("entry%05d", i)), []byte("small"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	gitTest(t, src, "add", ".")
	gitTest(t, src, "commit", "-m", "larger names fixture")
	head := gitTest(t, src, "rev-parse", "HEAD")
	s, e := Open(Config{Root: t.TempDir(), Repositories: map[string]string{"demo": src}, Principals: map[string]Principal{"alice": {Repositories: []string{"demo"}}}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Register(context.Background(), "alice", "demo", head); e != nil {
		t.Fatal(e)
	}
	p := Predicate{"exact_basename", []byte("absent"), 1}
	var rows []map[string]any
	for _, method := range []string{"fresh", "memoization", "receipts"} {
		if _, e = s.db.Exec("DELETE FROM cache"); e != nil {
			t.Fatal(e)
		}
		start := time.Now()
		if method == "receipts" {
			r, e := s.Search(context.Background(), "alice", SearchRequest{Repository: "demo", Snapshot: head, Predicate: p})
			if e != nil {
				t.Fatal(e)
			}
			rows = append(rows, map[string]any{"method": "receipts_cold", "entries": 4096, "total_ns": time.Since(start).Nanoseconds(), "stats": r.Stats})
			continue
		}
		snap, e := s.snapshot(context.Background(), "alice", "demo", head)
		if e != nil {
			t.Fatal(e)
		}
		for _, ent := range snap.Entries {
			v, _ := s.evaluate(context.Background(), "demo", p, ent)
			if method == "memoization" {
				if e = s.cache(context.Background(), s.cacheKey("alice", "demo", p, ent), v.Result); e != nil {
					t.Fatal(e)
				}
			}
		}
		rows = append(rows, map[string]any{"method": method + "_cold", "entries": 4096, "total_ns": time.Since(start).Nanoseconds()})
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	if e = os.WriteFile("../docs/larger-results.json", b, 0600); e != nil {
		t.Fatal(e)
	}
}
