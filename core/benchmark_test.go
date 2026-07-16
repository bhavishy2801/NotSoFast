package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Run explicitly: NSF_BENCH=1 go test ./core -run TestEarlyBenchmark -v -count=1.
// Both memoization and receipts use the SAME SQLite cache keys and evaluation code.
func TestEarlyBenchmark(t *testing.T) {
	if os.Getenv("NSF_BENCH") == "" {
		t.Skip("explicit benchmark only")
	}
	ctx := context.Background()
	type row struct {
		Size         int    `json:"entries"`
		Change       string `json:"change"`
		Predicate    string `json:"predicate"`
		Method       string `json:"method"`
		TotalNS      int64  `json:"total_ns"`
		ManifestNS   int64  `json:"manifest_ns"`
		LookupNS     int64  `json:"lookup_ns"`
		ReceiptNS    int64  `json:"receipt_ns"`
		Evaluated    int    `json:"evaluated"`
		Reused       int    `json:"reused"`
		Bytes        int64  `json:"bytes"`
		AllocBytes   uint64 `json:"alloc_bytes"`
		StorageBytes int64  `json:"storage_bytes"`
	}
	var results []row
	sizes := []int{32, 256}
	if os.Getenv("NSF_BENCH_SMALL") == "1" {
		sizes = []int{32}
	}
	for _, size := range sizes {
		src := t.TempDir()
		gitTest(t, src, "init")
		for i := 0; i < size; i++ {
			name := filepath.Join(src, fmt.Sprintf("f%05d.txt", i))
			if e := os.WriteFile(name, []byte(fmt.Sprintf("object %d ", i)+string(make([]byte, 4096))), 0600); e != nil {
				t.Fatal(e)
			}
		}
		gitTest(t, src, "add", ".")
		gitTest(t, src, "commit", "-m", "base")
		head := gitTest(t, src, "rev-parse", "HEAD")
		cfg := Config{Root: t.TempDir(), Repositories: map[string]string{"bench": src}, Principals: map[string]Principal{"bench": {Repositories: []string{"bench"}}}}
		s, e := Open(cfg)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.Register(ctx, "bench", "bench", head); e != nil {
			t.Fatal(e)
		}
		for _, kind := range []string{"exact_basename", "literal_bytes"} {
			wholeQueries := map[string]bool{}
			gitTest(t, src, "checkout", "--detach", head)
			p := Predicate{Kind: kind, Value: []byte("absent needle"), Version: 1}
			base := head
			for _, change := range []string{"cold", "unchanged", "light", "heavy"} {
				if change == "light" || change == "heavy" {
					count := 1
					if change == "heavy" {
						count = size * 3 / 4
					}
					for i := 0; i < count; i++ {
						if e = os.WriteFile(filepath.Join(src, fmt.Sprintf("f%05d.txt", i)), []byte(fmt.Sprintf("%s %s %d ", kind, change, i)+string(make([]byte, 4096))), 0600); e != nil {
							t.Fatal(e)
						}
					}
					gitTest(t, src, "add", ".")
					gitTest(t, src, "commit", "-m", change)
					base = gitTest(t, src, "rev-parse", "HEAD")
					pack, e := git(ctx, src, []byte(base+"\n"), "pack-objects", "--revs", "--stdout")
					if e != nil {
						t.Fatal(e)
					}
					if _, e = git(ctx, s.repo("bench"), pack, "index-pack", "--stdin"); e != nil {
						t.Fatal(e)
					}
					if _, e = git(ctx, s.repo("bench"), nil, "update-ref", branch, base); e != nil {
						t.Fatal(e)
					}
				}
				// Capture identical starting cache for D and E: E runs after restoring it.
				type pair struct{ key, value string }
				var cache []pair
				rows, e := s.db.Query("SELECT key,result FROM cache")
				if e != nil {
					t.Fatal(e)
				}
				for rows.Next() {
					var kv pair
					if e = rows.Scan(&kv.key, &kv.value); e != nil {
						t.Fatal(e)
					}
					cache = append(cache, kv)
				}
				rows.Close()
				for _, method := range []string{"fresh", "memoization", "receipts", "whole_query_memoization"} {
					if _, e = s.db.Exec("DELETE FROM cache"); e != nil {
						t.Fatal(e)
					}
					for _, kv := range cache {
						if _, e = s.db.Exec("INSERT INTO cache VALUES(?,?,?)", kv.key, kv.value, digest([]string{kv.key, kv.value})); e != nil {
							t.Fatal(e)
						}
					}
					var before, after runtime.MemStats
					runtime.ReadMemStats(&before)
					start := time.Now()
					v := row{Size: size, Change: change, Predicate: kind, Method: method}
					if method == "receipts" {
						r, e := s.Search(ctx, "bench", SearchRequest{Repository: "bench", Snapshot: base, Predicate: p})
						if e != nil {
							t.Fatal(e)
						}
						v.Evaluated = r.Stats.Evaluated
						v.Reused = r.Stats.Reused
						v.Bytes = r.Stats.Bytes
						v.ManifestNS = r.Stats.ManifestNS
						v.LookupNS = r.Stats.LookupNS
						v.ReceiptNS = time.Since(start).Nanoseconds() - r.Stats.TotalNS
					} else {
						snap, e := s.snapshot(ctx, "bench", "bench", base)
						if e != nil {
							t.Fatal(e)
						}
						v.ManifestNS = time.Since(start).Nanoseconds()
						entries := domain(snap, p, Scope{})
						if method == "whole_query_memoization" && wholeQueries[base] {
							v.Reused = len(entries)
							entries = nil
						}
						for _, ent := range entries {
							key := ""
							ts := time.Now()
							hit := false
							if method != "fresh" {
								key = s.cacheKey("bench", "bench", p, ent)
								_, hit = s.lookup(ctx, key)
							}
							v.LookupNS += time.Since(ts).Nanoseconds()
							if method != "fresh" && hit {
								v.Reused++
								continue
							}
							ev, n := s.evaluate(ctx, "bench", p, ent)
							if ev.Result == "UNEVALUATED" {
								t.Fatal(ev)
							}
							v.Bytes += n
							v.Evaluated++
							if method != "fresh" {
								if e = s.cache(ctx, key, ev.Result); e != nil {
									t.Fatal(e)
								}
							}
						}
						if method == "whole_query_memoization" {
							wholeQueries[base] = true
						}
					}
					v.TotalNS = time.Since(start).Nanoseconds()
					runtime.ReadMemStats(&after)
					v.AllocBytes = after.TotalAlloc - before.TotalAlloc
					v.StorageBytes, _ = dirSize(cfg.Root)
					results = append(results, v)
				}
			}
		}
		s.Close()
	}
	b, e := json.MarshalIndent(results, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	out := os.Getenv("NSF_BENCH_OUT")
	if out == "" {
		out = "../docs/early-results.json"
	}
	if e = os.WriteFile(out, b, 0600); e != nil {
		t.Fatal(e)
	}
	t.Logf("wrote %d actual measurements to %s", len(results), out)
}
