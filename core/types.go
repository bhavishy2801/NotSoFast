// Package core verifies exact absence over tracked immutable Git tree entries.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (e *Error) Error() string    { return e.Reason + ": " + e.Message }
func fail(code, msg string) error { return &Error{code, msg} }
func Code(e error) string {
	var x *Error
	if errors.As(e, &x) {
		return x.Reason
	}
	if e != nil {
		return "INTERNAL"
	}
	return ""
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Principal struct {
	Repositories []string `json:"repositories"`
	Write        bool     `json:"write"`
}
type Config struct {
	Root            string               `json:"root"`
	Repositories    map[string]string    `json:"repositories"`
	Principals      map[string]Principal `json:"principals"`
	Policies        map[string]Policy    `json:"policies"`
	Tokens          map[string]string    `json:"tokens"`
	Workers         int                  `json:"workers"`
	MaxBlobBytes    int64                `json:"max_blob_bytes"`
	MaxStorageBytes int64                `json:"max_storage_bytes"`
	MaxEntries      int                  `json:"max_entries"`
}
type Policy struct {
	Version           string `json:"version"`
	Kind              string `json:"kind"`
	Marker            []byte `json:"marker,omitempty"`
	DestinationPrefix string `json:"destination_prefix"`
	Scope             Scope  `json:"scope"`
	MaxBytes          int    `json:"max_bytes"`
}
type Entry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	OID  string `json:"oid"`
	Type string `json:"type"`
}
type Snapshot struct {
	Repository string  `json:"repository"`
	Commit     string  `json:"commit"`
	Tree       string  `json:"tree"`
	Algorithm  string  `json:"algorithm"`
	Version    int     `json:"version"`
	Manifest   string  `json:"manifest"`
	Entries    []Entry `json:"entries,omitempty"`
}
type Predicate struct {
	Kind    string `json:"kind"`
	Value   []byte `json:"value"`
	Version int    `json:"version"`
}
type Scope struct {
	Paths    []string `json:"paths,omitempty"`
	Prefixes []string `json:"prefixes,omitempty"`
	Type     string   `json:"type,omitempty"`
}
type Evaluation struct {
	Entry  Entry  `json:"entry"`
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
}
type Stats struct {
	Evaluated    int   `json:"evaluated"`
	Reused       int   `json:"reused"`
	Bytes        int64 `json:"bytes"`
	ManifestNS   int64 `json:"manifest_ns"`
	LookupNS     int64 `json:"lookup_ns"`
	EvaluationNS int64 `json:"evaluation_ns"`
	ReceiptNS    int64 `json:"receipt_ns"`
	TotalNS      int64 `json:"total_ns"`
}
type Receipt struct {
	ID          string       `json:"id"`
	Version     int          `json:"version"`
	Repository  string       `json:"repository"`
	Snapshot    string       `json:"snapshot"`
	Manifest    string       `json:"manifest"`
	Predicate   Predicate    `json:"predicate"`
	Scope       Scope        `json:"scope"`
	Access      string       `json:"access"`
	Adapter     string       `json:"adapter"`
	Complete    bool         `json:"complete"`
	Evaluations []Evaluation `json:"evaluations,omitempty"`
	Parents     []string     `json:"parents,omitempty"`
	Derivation  string       `json:"derivation"`
	Timestamp   string       `json:"timestamp"`
	Stats       Stats        `json:"stats"`
}
type SearchRequest struct {
	Repository string    `json:"repository"`
	Snapshot   string    `json:"snapshot"`
	Predicate  Predicate `json:"predicate"`
	Scope      Scope     `json:"scope"`
	Parents    []string  `json:"parents,omitempty"`
	Fresh      bool      `json:"fresh,omitempty"`
}
type Claim struct {
	Repository string    `json:"repository"`
	Snapshot   string    `json:"snapshot"`
	Predicate  Predicate `json:"predicate"`
	Scope      Scope     `json:"scope"`
	Receipts   []string  `json:"receipts"`
}
type Decision struct {
	Outcome   string   `json:"outcome"`
	Reason    string   `json:"reason"`
	Required  int      `json:"required"`
	Evaluated int      `json:"evaluated"`
	Missing   []string `json:"missing,omitempty"`
	Witnesses []string `json:"witnesses,omitempty"`
}
type CreateRequest struct {
	Repository    string   `json:"repository"`
	Snapshot      string   `json:"snapshot"`
	Operation     string   `json:"operation"`
	Policy        string   `json:"policy"`
	PolicyVersion string   `json:"policy_version"`
	Path          string   `json:"path"`
	Content       []byte   `json:"content"`
	Receipts      []string `json:"receipts"`
}
type Operation struct {
	Decisions map[string]Decision `json:"decisions"`
	Requester     string   `json:"requester"`
	ID            string   `json:"id"`
	Digest        string   `json:"digest"`
	PolicyVersion string   `json:"policy_version"`
	PolicyDigest  string   `json:"policy_digest"`
	Base          string   `json:"base"`
	Candidate     string   `json:"candidate"`
	Receipts      []string `json:"receipts"`
	Decision      Decision `json:"decision"`
	Outcome       string   `json:"outcome"`
}

func validPath(p string) bool {
	if p == "" || !utf8.ValidString(p) || strings.ContainsAny(p, "\x00\\") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}
func under(p, dir string) bool { return dir == "" || strings.HasPrefix(p, dir+"/") }
func (s Scope) validate() error {
	if s.Type != "" && s.Type != "regular" && s.Type != "tree" && s.Type != "symlink" && s.Type != "gitlink" {
		return fail("BAD_SCOPE", "unsupported type")
	}
	for _, p := range s.Paths {
		if !validPath(p) {
			return fail("BAD_SCOPE", "invalid path")
		}
	}
	for _, p := range s.Prefixes {
		if p != "" && !validPath(p) {
			return fail("BAD_SCOPE", "invalid prefix")
		}
	}
	return nil
}
func (s Scope) selects(e Entry) bool {
	if s.Type != "" && s.Type != e.Type {
		return false
	}
	if len(s.Paths)+len(s.Prefixes) == 0 {
		return true
	}
	for _, p := range s.Paths {
		if p == e.Path {
			return true
		}
	}
	for _, p := range s.Prefixes {
		if under(e.Path, p) {
			return true
		}
	}
	return false
}
func (p Predicate) validate() error {
	if p.Version != 1 || len(p.Value) == 0 || len(p.Value) > 65536 {
		return fail("BAD_PREDICATE", "version 1 and nonempty bounded value required")
	}
	switch p.Kind {
	case "exact_path":
		if !validPath(string(p.Value)) {
			return fail("BAD_PREDICATE", "invalid path")
		}
	case "exact_basename":
		if !validPath(string(p.Value)) || strings.Contains(string(p.Value), "/") {
			return fail("BAD_PREDICATE", "invalid basename")
		}
	case "literal_bytes":
	default:
		return fail("BAD_PREDICATE", "unsupported predicate")
	}
	return nil
}
func domain(s Snapshot, p Predicate, scope Scope) []Entry {
	var out []Entry
	for _, e := range s.Entries {
		if scope.selects(e) && (p.Kind != "literal_bytes" || e.Type == "regular") {
			out = append(out, e)
		}
	}
	return out
}
func nameMatch(p Predicate, e Entry) bool {
	if p.Kind == "exact_path" {
		return e.Path == string(p.Value)
	}
	return path.Base(e.Path) == string(p.Value)
}
func entryKey(e Entry) string { return digest(e) }
func oidValid(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil && s == strings.ToLower(s)
}
func requireOID(s string) error {
	if !oidValid(s) {
		return fail("BAD_SNAPSHOT", fmt.Sprintf("expected full lowercase commit ID, got length %d", len(s)))
	}
	return nil
}
