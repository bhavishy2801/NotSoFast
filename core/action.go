package core

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
)

func operationRef(user, id string) string {
	return "refs/notsofast/operations/" + digest([]string{user, id})
}
func intentRef(user, id string) string { return "refs/notsofast/intents/" + digest([]string{user, id}) }
func (s *Service) Operation(ctx context.Context, user, repo, id string) (Operation, error) {
	var op Operation
	if e := s.authorize(user, repo, false); e != nil {
		return op, e
	}
	ref := operationRef(user, id)
	exists, e := gitText(ctx, s.repo(repo), nil, "for-each-ref", "--format=%(objectname)", ref)
	if e != nil {
		return op, fail("UNRESOLVED", "operation references unavailable")
	}
	if exists == "" {
		exists, e = gitText(ctx, s.repo(repo), nil, "for-each-ref", "--format=%(objectname)", intentRef(user, id))
		if e != nil {
			return op, fail("UNRESOLVED", "intent references unavailable")
		}
		if exists == "" {
			return op, fail("NOT_FOUND", "operation unavailable")
		}
	}
	b, e := git(ctx, s.repo(repo), nil, "show", exists+":operation.json")
	if e != nil || json.Unmarshal(b, &op) != nil || op.Requester != user || op.ID != id || !oidValid(op.Candidate) || !oidValid(op.Base) {
		return Operation{}, fail("UNRESOLVED", "operation metadata unavailable")
	}
	message, e := gitText(ctx, s.repo(repo), nil, "show", "-s", "--format=%B", op.Candidate)
	parent, pe := gitText(ctx, s.repo(repo), nil, "show", "-s", "--format=%P", op.Candidate)
	if e != nil || pe != nil || parent != op.Base || message != "NotSoFast operation "+digest([]string{op.Requester, op.Digest}) {
		return Operation{}, fail("UNRESOLVED", "candidate operation binding invalid")
	}
	if _, e = git(ctx, s.repo(repo), nil, "merge-base", "--is-ancestor", op.Candidate, branch); e != nil {
		op.Outcome = "UNRESOLVED"
		return op, fail("UNRESOLVED", "intent exists but publication is not established")
	}
	return op, nil
}
func (s *Service) GuardedCreate(ctx context.Context, user string, q CreateRequest) (Operation, error) {
	// ponytail: serialize local mutations; Git CAS remains authoritative across interruption.
	s.mu.Lock()
	defer s.mu.Unlock()
	var op Operation
	if e := s.authorize(user, q.Repository, true); e != nil {
		return op, e
	}
	if len(q.Operation) == 0 || len(q.Operation) > 128 || len(q.Receipts) > 256 {
		return op, fail("BAD_REQUEST", "invalid operation identity or reference count")
	}
	q.Receipts = append([]string(nil), q.Receipts...)
	sort.Strings(q.Receipts)
	payload := digest(q)
	existing, e := s.Operation(ctx, user, q.Repository, q.Operation)
	if existing.Digest != "" && existing.Digest != payload {
		return op, fail("OPERATION_CONFLICT", "operation ID has a different payload")
	}
	if e == nil {
		if existing.Digest != payload {
			return op, fail("OPERATION_CONFLICT", "operation ID has a different payload")
		}
		return existing, nil
	}
	if Code(e) != "NOT_FOUND" {
		return op, e
	}
	p, ok := s.cfg.Policies[q.Policy]
	if !ok || q.PolicyVersion != p.Version {
		return op, fail("POLICY_CHANGED", "active policy version required")
	}
	if !validPath(q.Path) || !under(q.Path, p.DestinationPrefix) || len(q.Content) > p.MaxBytes {
		return op, fail("POLICY_DENIED", "destination or content limit")
	}
	for _, component := range strings.Split(q.Path, "/") {
		if strings.EqualFold(component, ".git") {
			return op, fail("POLICY_DENIED", "reserved destination component")
		}
	}
	if e = requireOID(q.Snapshot); e != nil {
		return op, e
	}
	head, e := s.Head(ctx, user, q.Repository)
	if e != nil {
		return op, e
	}
	if head != q.Snapshot {
		return op, fail("STATE_CHANGED", "managed head changed")
	}
	epoch, e := s.policyEpoch(ctx, q.Repository)
	if e != nil {
		return op, e
	}
	decisions, policySet, e := s.checkPolicies(ctx, user, q)
	if e != nil {
		return op, e
	}
	decision := decisions[q.Policy]
	snap, e := s.snapshot(ctx, user, q.Repository, head)
	if e != nil {
		return op, e
	}
	for _, ent := range snap.Entries {
		if ent.Path == q.Path || strings.HasPrefix(ent.Path, q.Path+"/") || (ent.Type != "tree" && strings.HasPrefix(q.Path, ent.Path+"/")) {
			return op, fail("DESTINATION_EXISTS", "entry or parent collision")
		}
	}
	if e = s.budget(); e != nil {
		return op, e
	}
	dir := s.repo(q.Repository)
	blob, e := gitText(ctx, dir, q.Content, "hash-object", "-w", "--stdin")
	if e != nil {
		return op, e
	}
	tree, e := s.addTree(ctx, dir, snap.Tree, strings.Split(q.Path, "/"), blob)
	if e != nil {
		return op, e
	}
	commit, e := gitText(ctx, dir, []byte("NotSoFast operation "+digest([]string{user, payload})+"\n"), "commit-tree", tree, "-p", head)
	if e != nil {
		return op, e
	}
	// Independently validate the exact raw tree delta before publishing.
	diff, e := git(ctx, dir, nil, "diff-tree", "--no-commit-id", "--no-renames", "-r", "--raw", "-z", head, commit)
	if e != nil {
		return op, e
	}
	parts := bytes.Split(diff, []byte{0})
	if len(parts) != 3 || string(parts[1]) != q.Path {
		return op, fail("CANDIDATE_INVALID", "unexpected change set")
	}
	fields := strings.Fields(string(parts[0]))
	if len(fields) != 5 || fields[0] != ":000000" || fields[1] != "100644" || fields[3] != blob || fields[4] != "A" {
		return op, fail("CANDIDATE_INVALID", "unexpected mode or content")
	}
	if _, e = s.manifest(ctx, q.Repository, commit); e != nil {
		return op, e
	}
	op = Operation{Requester: user, ID: q.Operation, Digest: payload, PolicyVersion: p.Version, PolicyDigest: policySet, Base: head, Candidate: commit, Receipts: q.Receipts, Decision: decision, Decisions: decisions, Outcome: "PUBLISHED"}
	body, _ := json.Marshal(op)
	metaBlob, e := gitText(ctx, dir, body, "hash-object", "-w", "--stdin")
	if e != nil {
		return Operation{}, e
	}
	metaTree, e := gitText(ctx, dir, []byte("100644 blob "+metaBlob+"\toperation.json\x00100644 blob "+epoch+"\tpolicy.json\x00"), "mktree", "-z")
	if e != nil {
		return Operation{}, e
	}
	metaCommit, e := gitText(ctx, dir, []byte("NotSoFast operation record\n"), "commit-tree", metaTree, "-p", commit)
	if e != nil {
		return Operation{}, e
	}
	// An immutable intent pins the candidate before the final reference transaction.
	if _, e = git(ctx, dir, []byte("create "+intentRef(user, q.Operation)+" "+metaCommit+"\n"), "update-ref", "--stdin"); e != nil {
		return Operation{}, fail("UNRESOLVED", "intent could not be confirmed")
	}
	tx := "start\nupdate " + branch + " " + commit + " " + head + "\ncreate " + operationRef(user, q.Operation) + " " + metaCommit + "\nverify " + policyRef + " " + epoch + "\nprepare\ncommit\n"
	if _, e = git(ctx, dir, []byte(tx), "update-ref", "--stdin"); e != nil {
		recovered, err := s.Operation(context.WithoutCancel(ctx), user, q.Repository, q.Operation)
		if err == nil && recovered.Digest == payload {
			return recovered, nil
		}
		recoveryErr := err
		newHead, err := s.Head(context.WithoutCancel(ctx), user, q.Repository)
		if err == nil && newHead != head {
			return Operation{}, fail("STATE_CHANGED", "conditional publication failed")
		}
		if _, err = s.policyEpoch(context.WithoutCancel(ctx), q.Repository); Code(err) == "POLICY_CHANGED" {
			return Operation{}, err
		}
		if Code(recoveryErr) == "UNRESOLVED" {
			return Operation{}, recoveryErr
		}
		return Operation{}, fail("UNRESOLVED", "publication could not be confirmed; inspect operation before retrying")
	}
	return op, nil
}

// Rebuild only ancestor trees, preserving arbitrary supported entry names without an index or checkout.
func (s *Service) addTree(ctx context.Context, dir, tree string, components []string, blob string) (string, error) {
	var rows [][]byte
	if tree != "" {
		b, e := git(ctx, dir, nil, "ls-tree", "-z", tree)
		if e != nil {
			return "", e
		}
		for _, row := range bytes.Split(b, []byte{0}) {
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}
	name := components[0]
	subtree := ""
	kept := make([][]byte, 0, len(rows)+1)
	for _, row := range rows {
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) != 2 {
			return "", fail("MANIFEST_ERROR", "invalid tree")
		}
		if string(parts[1]) == name {
			f := strings.Fields(string(parts[0]))
			if len(components) == 1 || len(f) != 3 || f[1] != "tree" {
				return "", fail("DESTINATION_EXISTS", "collision")
			}
			subtree = f[2]
		} else {
			kept = append(kept, row)
		}
	}
	row := "100644 blob " + blob + "\t" + name
	if len(components) > 1 {
		child, e := s.addTree(ctx, dir, subtree, components[1:], blob)
		if e != nil {
			return "", e
		}
		row = "040000 tree " + child + "\t" + name
	}
	kept = append(kept, []byte(row))
	input := append(bytes.Join(kept, []byte{0}), 0)
	return gitText(ctx, dir, input, "mktree", "-z", "--missing")
}
