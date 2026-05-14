package core

import (
	"bytes"
	"context"
	"sort"
	"time"
)

func (s *Service) evaluate(ctx context.Context, repo string, p Predicate, e Entry) (Evaluation, int64) {
	v := Evaluation{Entry: e, Result: "NO_MATCH"}
	if ctx.Err() != nil {
		v.Result = "UNEVALUATED"
		v.Reason = "CANCELLED"
		return v, 0
	}
	if p.Kind != "literal_bytes" {
		if nameMatch(p, e) {
			v.Result = "MATCH"
		}
		return v, 0
	}
	b, err := s.blob(ctx, repo, e.OID)
	if err != nil {
		v.Result = "UNEVALUATED"
		v.Reason = Code(err)
		return v, 0
	}
	if bytes.Contains(b, p.Value) {
		v.Result = "MATCH"
	}
	return v, int64(len(b))
}
func (s *Service) Search(ctx context.Context, user string, q SearchRequest) (Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.search(ctx, user, q)
}
func (s *Service) search(ctx context.Context, user string, q SearchRequest) (Receipt, error) {
	start := time.Now()
	var r Receipt
	if e := q.Predicate.validate(); e != nil {
		return r, e
	}
	if e := q.Scope.validate(); e != nil {
		return r, e
	}
	if len(q.Parents) > 256 {
		return r, fail("LIMIT", "too many receipt references")
	}
	snap, e := s.snapshot(ctx, user, q.Repository, q.Snapshot)
	if e != nil {
		return r, e
	}
	manifestNS := time.Since(start).Nanoseconds()
	if e = s.budget(); e != nil {
		return r, e
	}
	inherited := map[string]Evaluation{}
	for _, id := range q.Parents {
		old, e := s.Receipt(ctx, user, id)
		if e != nil {
			return r, e
		}
		if old.Repository != q.Repository || digest(old.Predicate) != digest(q.Predicate) {
			return r, fail("INVALID_EVIDENCE", "incompatible parent")
		}
		oldSnap, e := s.snapshot(ctx, user, old.Repository, old.Snapshot)
		if e != nil {
			return r, fail("INVALID_EVIDENCE", "parent snapshot unavailable")
		}
		if e = validateReceipt(old, oldSnap); e != nil {
			return r, e
		}
		for _, v := range old.Evaluations {
			if v.Result == "UNEVALUATED" {
				continue
			}
			k := entryKey(v.Entry)
			if prev, ok := inherited[k]; ok && prev.Result != v.Result {
				return r, fail("INVALID_EVIDENCE", "conflicting evaluations")
			}
			inherited[k] = v
		}
	}
	r = Receipt{Version: 1, Repository: q.Repository, Snapshot: q.Snapshot, Manifest: snap.Manifest, Predicate: q.Predicate, Scope: q.Scope, Access: s.access(user, q.Repository), Adapter: "git-v1", Complete: true, Parents: append([]string(nil), q.Parents...), Derivation: "fresh", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
	if !q.Fresh {
		r.Derivation = "memoized"
	}
	if len(q.Parents) > 0 {
		r.Derivation = "refresh"
	}
	r.Stats.ManifestNS = manifestNS
	for _, ent := range domain(snap, q.Predicate, q.Scope) {
		key := s.cacheKey(user, q.Repository, q.Predicate, ent)
		v := Evaluation{Entry: ent}
		lookup := time.Now()
		old, ok := inherited[entryKey(ent)]
		if !q.Fresh && ok {
			v = old
			r.Stats.Reused++
		} else if cached, hit := s.lookup(ctx, key); !q.Fresh && hit {
			v.Result = cached
			r.Stats.Reused++
		}
		r.Stats.LookupNS += time.Since(lookup).Nanoseconds()
		if v.Result == "" {
			now := time.Now()
			var n int64
			v, n = s.evaluate(ctx, q.Repository, q.Predicate, ent)
			r.Stats.Evaluated++
			r.Stats.Bytes += n
			r.Stats.EvaluationNS += time.Since(now).Nanoseconds()
			if v.Result != "UNEVALUATED" && !q.Fresh {
				if e = s.cache(ctx, key, v.Result); e != nil {
					return Receipt{}, e
				}
			}
		}
		if v.Result == "UNEVALUATED" {
			r.Complete = false
		}
		r.Evaluations = append(r.Evaluations, v)
	}
	r.Stats.TotalNS = time.Since(start).Nanoseconds()
	// A cancelled caller gets no authoritative new receipt. Previously stored parents remain usable.
	if e = ctx.Err(); e != nil {
		return Receipt{}, fail("CANCELLED", "search cancelled")
	}
	if e = s.save(ctx, &r); e != nil {
		return Receipt{}, e
	}
	return r, nil
}
func validateReceipt(r Receipt, snap Snapshot) error {
	if r.Manifest != snap.Manifest || r.Predicate.validate() != nil || r.Scope.validate() != nil {
		return fail("INVALID_EVIDENCE", "receipt identity or semantics mismatch")
	}
	allowed := map[string]bool{}
	for _, e := range domain(snap, r.Predicate, r.Scope) {
		allowed[entryKey(e)] = true
	}
	seen := map[string]bool{}
	complete := true
	for _, v := range r.Evaluations {
		k := entryKey(v.Entry)
		if !allowed[k] || seen[k] || (v.Result != "MATCH" && v.Result != "NO_MATCH" && v.Result != "UNEVALUATED") || (v.Result == "UNEVALUATED" && v.Reason == "") {
			return fail("INVALID_EVIDENCE", "invalid evaluation set")
		}
		seen[k] = true
		if v.Result == "UNEVALUATED" {
			complete = false
		}
	}
	if len(seen) != len(allowed) {
		complete = false
	}
	if complete != r.Complete {
		return fail("INVALID_EVIDENCE", "inconsistent completion")
	}
	return nil
}
func (s *Service) Verify(ctx context.Context, user string, c Claim) (Decision, error) {
	d := Decision{Outcome: "INVALID_EVIDENCE"}
	if e := c.Predicate.validate(); e != nil {
		return d, e
	}
	if e := c.Scope.validate(); e != nil {
		return d, e
	}
	if len(c.Receipts) > 256 {
		return d, fail("LIMIT", "too many receipts")
	}
	snap, e := s.snapshot(ctx, user, c.Repository, c.Snapshot)
	if e != nil {
		return d, e
	}
	evaluated := map[string]Evaluation{}
	// Validate every submitted receipt before considering any matching witness.
	for _, id := range c.Receipts {
		r, e := s.Receipt(ctx, user, id)
		if e != nil {
			return d, e
		}
		if r.Repository != c.Repository || r.Snapshot != c.Snapshot || digest(r.Predicate) != digest(c.Predicate) {
			return d, fail("INVALID_EVIDENCE", "incompatible receipt")
		}
		if e = validateReceipt(r, snap); e != nil {
			return d, e
		}
		for _, v := range r.Evaluations {
			if v.Result == "UNEVALUATED" {
				continue
			}
			k := entryKey(v.Entry)
			if prev, ok := evaluated[k]; ok && prev.Result != v.Result {
				return d, fail("INVALID_EVIDENCE", "conflicting evaluations")
			}
			evaluated[k] = v
		}
	}
	entries := domain(snap, c.Predicate, c.Scope)
	d.Required = len(entries)
	for _, ent := range entries {
		v, ok := evaluated[entryKey(ent)]
		if !ok {
			d.Missing = append(d.Missing, ent.Path)
			continue
		}
		d.Evaluated++
		if v.Result == "MATCH" {
			d.Witnesses = append(d.Witnesses, ent.Path)
		}
	}
	switch {
	case len(d.Witnesses) > 0:
		d.Outcome = "REFUTED"
		d.Reason = "MATCHING_WITNESS"
	case len(d.Missing) > 0:
		d.Outcome = "UNKNOWN"
		d.Reason = "MISSING_COVERAGE"
	default:
		d.Outcome = "SUPPORTED"
		d.Reason = "COMPLETE_ABSENCE"
	}
	return d, nil
}
func (s *Service) SearchMissing(ctx context.Context, user string, c Claim) (Receipt, error) {
	if _, e := s.Verify(ctx, user, c); e != nil {
		return Receipt{}, e
	}
	return s.Search(ctx, user, SearchRequest{Repository: c.Repository, Snapshot: c.Snapshot, Predicate: c.Predicate, Scope: c.Scope, Parents: c.Receipts})
}
func (s *Service) Compose(ctx context.Context, user string, c Claim) (Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, e := s.Verify(ctx, user, c); e != nil {
		return Receipt{}, e
	}
	snap, e := s.snapshot(ctx, user, c.Repository, c.Snapshot)
	if e != nil {
		return Receipt{}, e
	}
	if e = s.budget(); e != nil {
		return Receipt{}, e
	}
	values := map[string]Evaluation{}
	for _, id := range c.Receipts {
		r, e := s.Receipt(ctx, user, id)
		if e != nil {
			return Receipt{}, e
		}
		for _, v := range r.Evaluations {
			if c.Scope.selects(v.Entry) {
				k := entryKey(v.Entry)
				prev, ok := values[k]
				if !ok || prev.Result == "UNEVALUATED" {
					values[k] = v
				}
			}
		}
	}
	r := Receipt{Version: 1, Repository: c.Repository, Snapshot: c.Snapshot, Manifest: snap.Manifest, Predicate: c.Predicate, Scope: c.Scope, Access: s.access(user, c.Repository), Adapter: "git-v1", Parents: append([]string(nil), c.Receipts...), Derivation: "composition", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Complete: true}
	for _, v := range values {
		r.Evaluations = append(r.Evaluations, v)
		if v.Result == "UNEVALUATED" {
			r.Complete = false
		}
	}
	sort.Slice(r.Evaluations, func(i, j int) bool { return r.Evaluations[i].Entry.Path < r.Evaluations[j].Entry.Path })
	if len(r.Evaluations) != len(domain(snap, c.Predicate, c.Scope)) {
		r.Complete = false
	}
	if e = s.save(ctx, &r); e != nil {
		return Receipt{}, e
	}
	return r, nil
}
