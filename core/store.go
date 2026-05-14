package core

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Service struct {
	cfg   Config
	db    *sql.DB
	lease *sql.DB
	mu    sync.Mutex
}

func Open(cfg Config) (*Service, error) {
	// Copy operator configuration so callers cannot mutate active policy maps.
	b, e := json.Marshal(cfg)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(b, &cfg); e != nil {
		return nil, e
	}
	if cfg.Root == "" {
		return nil, fail("CONFIG", "root required")
	}
	cfg.Root, e = filepath.Abs(cfg.Root)
	if e != nil {
		return nil, e
	}
	if cfg.Workers == 0 {
		cfg.Workers = 4
	}
	if cfg.MaxBlobBytes == 0 {
		cfg.MaxBlobBytes = 4 << 20
	}
	if cfg.MaxStorageBytes == 0 {
		cfg.MaxStorageBytes = 512 << 20
	}
	if cfg.MaxEntries == 0 {
		cfg.MaxEntries = 100000
	}
	if cfg.Workers < 1 || cfg.Workers > 64 || cfg.MaxBlobBytes < 1 || cfg.MaxBlobBytes > 32<<20 || cfg.MaxStorageBytes < 1 || cfg.MaxEntries < 1 || cfg.MaxEntries > 1000000 {
		return nil, fail("CONFIG", "invalid limits")
	}
	for _, p := range cfg.Policies {
		if p.Version == "" || p.MaxBytes < 1 || p.MaxBytes > int(cfg.MaxBlobBytes) || (p.Kind != "exact_basename" && p.Kind != "literal_bytes") || p.Scope.validate() != nil || (p.DestinationPrefix != "" && !validPath(p.DestinationPrefix)) || (p.Kind == "literal_bytes" && len(p.Marker) == 0) {
			return nil, fail("CONFIG", "invalid policy")
		}
	}
	if e = os.MkdirAll(filepath.Join(cfg.Root, "repos"), 0700); e != nil {
		return nil, e
	}
	lease, e := sql.Open("sqlite3", filepath.Join(cfg.Root, "instance.db")+"?_busy_timeout=0")
	if e != nil {
		return nil, e
	}
	lease.SetMaxOpenConns(1)
	if _, e = lease.Exec("BEGIN EXCLUSIVE"); e != nil {
		lease.Close()
		return nil, fail("INSTANCE_BUSY", "another service instance owns this state directory")
	}
	db, e := sql.Open("sqlite3", filepath.Join(cfg.Root, "evidence.db")+"?_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL")
	if e != nil {
		lease.Close()
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`CREATE TABLE IF NOT EXISTS receipts(id TEXT PRIMARY KEY, body BLOB NOT NULL); CREATE TABLE IF NOT EXISTS cache(key TEXT PRIMARY KEY,result TEXT NOT NULL CHECK(result IN ('MATCH','NO_MATCH')), integrity TEXT NOT NULL);`)
	if e != nil {
		db.Close()
		lease.Close()
		return nil, e
	}
	return &Service{cfg: cfg, db: db, lease: lease}, nil
}
func (s *Service) Close() error { e := s.db.Close(); s.lease.Close(); return e }
func (s *Service) authorize(user, repo string, write bool) error {
	p, ok := s.cfg.Principals[user]
	if !ok || write && !p.Write {
		return fail("FORBIDDEN", "resource unavailable")
	}
	if _, ok = s.cfg.Repositories[repo]; !ok {
		return fail("FORBIDDEN", "resource unavailable")
	}
	for _, id := range p.Repositories {
		if id == repo {
			return nil
		}
	}
	return fail("FORBIDDEN", "resource unavailable")
}
func (s *Service) access(user, repo string) string {
	return digest(struct {
		User, Repository string
		Principal        Principal
	}{user, repo, s.cfg.Principals[user]})
}
func (s *Service) budget() error {
	n, e := dirSize(s.cfg.Root)
	if e != nil {
		return e
	}
	if n+maxGitOutput > s.cfg.MaxStorageBytes {
		return fail("STORAGE_LIMIT", "storage reserve exhausted; records are retained")
	}
	return nil
}
func receiptID(r Receipt) string { r.ID = ""; return digest(r) }
func (s *Service) save(ctx context.Context, r *Receipt) error {
	r.ID = receiptID(*r)
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if len(b) > 32<<20 {
		return fail("LIMIT", "receipt size limit")
	}
	_, e = s.db.ExecContext(ctx, "INSERT INTO receipts(id,body) VALUES(?,?)", r.ID, b)
	return e
}
func (s *Service) Receipt(ctx context.Context, user, id string) (Receipt, error) {
	var r Receipt
	var b []byte
	if len(id) != 64 {
		return r, fail("INVALID_EVIDENCE", "receipt unavailable")
	}
	if e := s.db.QueryRowContext(ctx, "SELECT body FROM receipts WHERE id=?", id).Scan(&b); e != nil {
		return r, fail("INVALID_EVIDENCE", "receipt unavailable")
	}
	if json.Unmarshal(b, &r) != nil || r.ID != id || receiptID(r) != id || r.Version != 1 || r.Adapter != "git-v1" || s.authorize(user, r.Repository, false) != nil || r.Access != s.access(user, r.Repository) {
		return Receipt{}, fail("INVALID_EVIDENCE", "receipt unavailable")
	}
	return r, nil
}
func (s *Service) cacheKey(user, repo string, p Predicate, e Entry) string {
	input := e.Path
	if p.Kind == "literal_bytes" {
		input = e.OID
	}
	return digest(struct {
		Access, Input, Mode string
		Predicate           Predicate
		Version             int
	}{s.access(user, repo), input, e.Mode, p, 1})
}
func (s *Service) lookup(ctx context.Context, key string) (string, bool) {
	var v, integrity string
	e := s.db.QueryRowContext(ctx, "SELECT result,integrity FROM cache WHERE key=?", key).Scan(&v, &integrity)
	return v, e == nil && (v == "MATCH" || v == "NO_MATCH") && integrity == digest([]string{key, v})
}
func (s *Service) cache(ctx context.Context, key, value string) error {
	_, e := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO cache(key,result,integrity) VALUES(?,?,?)", key, value, digest([]string{key, value}))
	if e != nil {
		return e
	}
	v, ok := s.lookup(ctx, key)
	if !ok || v != value {
		return fail("INTEGRITY", "conflicting evaluations")
	}
	return nil
}
func (s *Service) Authenticate(token string) (string, bool) {
	for t, p := range s.cfg.Tokens {
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 && len(strings.TrimSpace(t)) >= 24 {
			_, ok := s.cfg.Principals[p]
			return p, ok
		}
	}
	return "", false
}
