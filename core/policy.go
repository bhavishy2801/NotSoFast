package core

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"sort"
)

const policyRef = "refs/notsofast/policy"

func (s *Service) policyBody() []byte {
	var repos []string
	for id := range s.cfg.Repositories {
		repos = append(repos, id)
	}
	sort.Strings(repos)
	b, _ := json.Marshal(struct {
		Policies     map[string]Policy
		Principals   map[string]Principal
		Repositories []string
	}{s.cfg.Policies, s.cfg.Principals, repos})
	return b
}
func (s *Service) activatePolicy(ctx context.Context, repo string) error {
	return s.activatePolicyDirectory(ctx, s.repo(repo))
}
func (s *Service) activatePolicyDirectory(ctx context.Context, dir string) error {
	oid, e := gitText(ctx, dir, s.policyBody(), "hash-object", "-w", "--stdin")
	if e != nil {
		return e
	}
	_, e = git(ctx, dir, nil, "update-ref", policyRef, oid)
	return e
}
func (s *Service) policyEpoch(ctx context.Context, repo string) (string, error) {
	expected, e := gitText(ctx, s.repo(repo), s.policyBody(), "hash-object", "--stdin")
	if e != nil {
		return "", e
	}
	actual, e := gitText(ctx, s.repo(repo), nil, "rev-parse", "--verify", policyRef)
	if e != nil || actual != expected {
		return "", fail("POLICY_CHANGED", "operator policy epoch changed")
	}
	return expected, nil
}

// Applicable conventions are conjunctive: choosing a policy never disables another.
func (s *Service) checkPolicies(ctx context.Context, user string, q CreateRequest) (map[string]Decision, string, error) {
	decisions := map[string]Decision{}
	policies := map[string]Policy{}
	groups := map[string][]string{}
	snap, e := s.snapshot(ctx, user, q.Repository, q.Snapshot)
	if e != nil {
		return nil, "", e
	}
	for _, id := range q.Receipts {
		r, e := s.Receipt(ctx, user, id)
		if e != nil {
			return nil, "", e
		}
		if r.Repository != q.Repository || r.Snapshot != q.Snapshot {
			return nil, "", fail("INVALID_EVIDENCE", "incompatible receipt")
		}
		if e = validateReceipt(r, snap); e != nil {
			return nil, "", e
		}
		key := digest(r.Predicate)
		groups[key] = append(groups[key], id)
	}
	var names []string
	for name, p := range s.cfg.Policies {
		if under(q.Path, p.DestinationPrefix) {
			names = append(names, name)
			policies[name] = p
		}
	}
	sort.Strings(names)
	for _, name := range names {
		p := policies[name]
		ent := Entry{Path: q.Path, Type: "regular", Mode: "100644"}
		if len(q.Content) > p.MaxBytes || !p.Scope.selects(ent) {
			return nil, "", fail("POLICY_DENIED", "candidate violates applicable convention")
		}
		pred := Predicate{Kind: p.Kind, Version: 1, Value: []byte(path.Base(q.Path))}
		if p.Kind == "literal_bytes" {
			pred.Value = p.Marker
			if !bytes.Contains(q.Content, p.Marker) {
				return nil, "", fail("POLICY_DENIED", "candidate lacks required literal marker")
			}
		}
		d, e := s.Verify(ctx, user, Claim{Repository: q.Repository, Snapshot: q.Snapshot, Predicate: pred, Scope: p.Scope, Receipts: groups[digest(pred)]})
		if e != nil {
			return nil, "", e
		}
		if d.Outcome != "SUPPORTED" {
			return nil, "", fail(d.Outcome, d.Reason)
		}
		decisions[name] = d
	}
	return decisions, digest(policies), nil
}
