// Package protocol handles transport decoding; all decisions live in core.
package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"notsofast/core"
)

func decode(body []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return &core.Error{Reason: "BAD_REQUEST", Message: "invalid request fields"}
	}
	if d.Decode(new(any)) != io.EOF {
		return &core.Error{Reason: "BAD_REQUEST", Message: "one JSON object required"}
	}
	return nil
}
func Dispatch(ctx context.Context, s *core.Service, user, op string, body json.RawMessage, expand bool) (any, error) {
	switch op {
	case "register", "snapshot", "head":
		var q struct {
			Repository string `json:"repository"`
			Snapshot   string `json:"snapshot"`
		}
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		if op == "head" {
			h, e := s.Head(ctx, user, q.Repository)
			return map[string]string{"snapshot": h}, e
		}
		var snap core.Snapshot
		var e error
		if op == "register" {
			snap, e = s.Register(ctx, user, q.Repository, q.Snapshot)
		} else {
			snap, e = s.Snapshot(ctx, user, q.Repository, q.Snapshot)
		}
		count := len(snap.Entries)
		if !expand {
			snap.Entries = nil
		}
		return struct {
			Snapshot core.Snapshot `json:"snapshot"`
			Entries  int           `json:"entry_count"`
			Universe string        `json:"universe"`
		}{snap, count, "tracked tree entries; excludes working tree, symlink targets, submodule contents and LFS payloads"}, e
	case "search", "refresh":
		var q core.SearchRequest
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		r, e := s.Search(ctx, user, q)
		return compactReceipt(r, expand), e
	case "verify", "compose", "search_missing":
		var q core.Claim
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		if op == "verify" {
			d, e := s.Verify(ctx, user, q)
			return d, e
		}
		var r core.Receipt
		var e error
		if op == "compose" {
			r, e = s.Compose(ctx, user, q)
		} else {
			r, e = s.SearchMissing(ctx, user, q)
		}
		return compactReceipt(r, expand), e
	case "receipt":
		var q struct {
			ID string `json:"id"`
		}
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		r, e := s.Receipt(ctx, user, q.ID)
		return compactReceipt(r, expand), e
	case "guarded_create":
		var q core.CreateRequest
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		return s.GuardedCreate(ctx, user, q)
	case "operation":
		var q struct {
			Repository string `json:"repository"`
			ID         string `json:"id"`
		}
		if e := decode(body, &q); e != nil {
			return nil, e
		}
		return s.Operation(ctx, user, q.Repository, q.ID)
	default:
		return nil, &core.Error{Reason: "BAD_REQUEST", Message: "unknown operation"}
	}
}
func compactReceipt(r core.Receipt, expand bool) any {
	count := len(r.Evaluations)
	if !expand {
		r.Evaluations = nil
	}
	return struct {
		Receipt     core.Receipt `json:"receipt"`
		Evaluations int          `json:"evaluation_count"`
	}{r, count}
}
