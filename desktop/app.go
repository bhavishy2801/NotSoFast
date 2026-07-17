// Package desktop provides the trusted local operator UI. The network API is unchanged.
package desktop

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"notsofast/core"
	"notsofast/protocol"
)

//go:embed web/*
var assets embed.FS

type App struct {
	Root, Host, token string
	mu                sync.Mutex
	db, lease         *sql.DB
	service           *core.Service
	source            string
	cfg               core.Config
	model             ModelConfig
	files             http.Handler
	Quit              func()
}

type Event struct {
	ID      int64           `json:"id"`
	Time    string          `json:"time"`
	Action  string          `json:"action"`
	State   string          `json:"state"`
	Request json.RawMessage `json:"request"`
	Result  json.RawMessage `json:"result"`
}

func New(root, token string) (*App, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	lease, err := sql.Open("sqlite3", filepath.Join(root, "desktop-instance.db")+"?_busy_timeout=0")
	if err != nil {
		return nil, err
	}
	lease.SetMaxOpenConns(1)
	if _, err = lease.Exec("BEGIN EXCLUSIVE"); err != nil {
		lease.Close()
		return nil, fmt.Errorf("NotSoFast is already running")
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "desktop.db")+"?_journal_mode=WAL&_synchronous=FULL&_busy_timeout=5000")
	if err != nil {
		lease.Close()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA max_page_count=16384; PRAGMA wal_autocheckpoint=256; PRAGMA journal_size_limit=8388608;"); err != nil {
		db.Close()
		lease.Close()
		return nil, err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT); CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY,source TEXT,time TEXT,action TEXT,state TEXT,request BLOB,result BLOB); CREATE TABLE IF NOT EXISTS model_runs(id INTEGER PRIMARY KEY,time TEXT,result BLOB);`); err != nil {
		db.Close()
		lease.Close()
		return nil, err
	}
	sub, _ := fs.Sub(assets, "web")
	a := &App{Root: root, token: token, db: db, lease: lease, files: http.FileServer(http.FS(sub))}
	var modelBody string
	if db.QueryRow("SELECT value FROM settings WHERE key='model'").Scan(&modelBody) == nil {
		_ = json.Unmarshal([]byte(modelBody), &a.model)
		a.model.Key = ""
	}
	var source string
	if err = db.QueryRow("SELECT value FROM settings WHERE key='source'").Scan(&source); err == nil && source != "" {
		// A disconnected drive must not prevent the UI from opening for another workspace.
		_ = a.connect(context.Background(), source, false)
	}
	return a, nil
}
func (a *App) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.service != nil {
		a.service.Close()
		a.service = nil
	}
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}
	if a.lease != nil {
		a.lease.Close()
		a.lease = nil
	}
}

func localGit(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=" + "/dev/null", "-c", "core.fsmonitor=false", "-c", "protocol.allow=never", "-C", dir}, args...)...)
	quiet(cmd)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(v), "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+"/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=", "GIT_AUTHOR_NAME=NotSoFast", "GIT_AUTHOR_EMAIL=local@notsofast.invalid", "GIT_COMMITTER_NAME=NotSoFast", "GIT_COMMITTER_EMAIL=local@notsofast.invalid")
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Git could not %s; select a local Git repository with at least one commit", args[0])
	}
	return strings.TrimSpace(string(b)), nil
}
func (a *App) demo(ctx context.Context) (string, error) {
	dir := filepath.Join(a.Root, "sample-repository")
	if _, err := localGit(ctx, dir, "rev-parse", "--verify", "HEAD^{commit}"); err == nil {
		return dir, nil
	}
	for _, d := range []string{"src", "config", "docs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0700); err != nil {
			return "", err
		}
	}
	for name, body := range map[string]string{"README.md": "# Orbit workspace\nA safe sample repository for NotSoFast.\n", "src/app.go": "package main\n\nfunc main() {}\n", "config/database.yaml": "database: orbit\n", "docs/architecture.md": "# Architecture\nEvidence before action.\n", "src/routes.go": "package main\n// Routes are defined here.\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			return "", err
		}
	}
	for _, args := range [][]string{{"init"}, {"add", "."}, {"commit", "-m", "Create sample workspace"}} {
		if _, err := localGit(ctx, dir, args...); err != nil {
			return "", err
		}
	}
	return dir, nil
}
func (a *App) connect(ctx context.Context, source string, refresh bool) error {
	abs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("folder unavailable: %w", err)
	}
	if a.service != nil && abs == a.source && !refresh {
		return nil
	}
	commit, err := localGit(ctx, abs, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if abs == a.source && a.service != nil {
		_, err = a.service.Register(ctx, "local", "workspace", commit)
		return err
	}
	key := sha256.Sum256([]byte(abs))
	root := filepath.Join(a.Root, "workspaces", hex.EncodeToString(key[:16]))
	cfg := core.Config{Root: root, Repositories: map[string]string{"workspace": abs}, Principals: map[string]core.Principal{"local": {Repositories: []string{"workspace"}, Write: true}}, Policies: map[string]core.Policy{
		"unique": {Version: "1", Kind: "exact_basename", MaxBytes: 65536},
		"marker": {Version: "1", Kind: "literal_bytes", DestinationPrefix: "markers", Marker: []byte("owned: demo"), MaxBytes: 65536},
	}}
	s, err := core.Open(cfg)
	if err != nil {
		return err
	}
	if _, err = s.Register(ctx, "local", "workspace", commit); err != nil {
		s.Close()
		return err
	}
	if _, err = a.db.Exec("INSERT OR REPLACE INTO settings(key,value) VALUES('source',?)", abs); err != nil {
		s.Close()
		return err
	}
	if a.service != nil {
		a.service.Close()
	}
	a.service = s
	a.source = abs
	a.cfg = cfg
	return nil
}
func (a *App) workspaceID() string {
	h := sha256.Sum256([]byte(a.source))
	return hex.EncodeToString(h[:])
}
func (a *App) status(ctx context.Context) (any, error) {
	events := []Event{}
	rows, err := a.db.Query("SELECT id,time,action,state,request,result FROM events WHERE source=? AND (state='PENDING' OR id IN (SELECT id FROM events WHERE source=? ORDER BY id DESC LIMIT 200)) ORDER BY id DESC", a.source, a.source)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.ID, &e.Time, &e.Action, &e.State, &e.Request, &e.Result); err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"connected": a.service != nil, "source": a.source, "workspace_id": a.workspaceID(), "events": events, "policies": a.cfg.Policies, "state_root": a.cfg.Root, "version": "0.2.0"}
	safeModel := a.model
	safeModel.Key = ""
	out["model"] = safeModel
	out["model_key_in_session"] = a.model.Key != ""
	if a.service != nil {
		head, err := a.service.Head(ctx, "local", "workspace")
		if err != nil {
			return nil, err
		}
		snap, err := a.service.Snapshot(ctx, "local", "workspace", head)
		if err != nil {
			return nil, err
		}
		out["entry_count"] = len(snap.Entries)
		snap.Entries = nil
		out["snapshot"] = snap
	}
	return out, nil
}
func readJSON(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("one JSON value required")
	}
	return nil
}
func reply(w http.ResponseWriter, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"reason": core.Code(err), "message": err.Error()}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"result": result})
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 2<<20))
			_ = r.Body.Close()
		}
	}()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	if r.Host != a.Host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+a.Host) {
		http.Error(w, "Untrusted origin", 403)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		a.files.ServeHTTP(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-NSF-Session")), []byte(a.token)) != 1 {
		http.Error(w, "Session required; reopen NotSoFast", 401)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if r.URL.Path == "/api/ping" {
		reply(w, map[string]bool{"alive": true}, nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	// ponytail: one interactive operation at a time; introduce a task queue only if multi-user operation is added.
	if !a.mu.TryLock() {
		http.Error(w, "Workspace busy", 429)
		return
	}
	defer a.mu.Unlock()
	if (strings.HasPrefix(r.URL.Path, "/api/call/") || r.URL.Path == "/api/export") && r.Header.Get("X-NSF-Workspace") != a.workspaceID() {
		http.Error(w, "Workspace changed in another window. Refresh this window before continuing.", 409)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	var out any
	var err error
	switch r.URL.Path {
	case "/api/status":
		out, err = a.status(ctx)
	case "/api/model/discover":
		out = discoverModels(ctx)
	case "/api/model/configure":
		var cfg ModelConfig
		if err = readJSON(r, &cfg); err == nil {
			err = cfg.validate()
		}
		if err == nil {
			safe := cfg
			safe.Key = ""
			b, _ := json.Marshal(safe)
			_, err = a.db.Exec("INSERT OR REPLACE INTO settings(key,value) VALUES('model',?)", string(b))
			if err == nil {
				a.model = cfg
				out = map[string]bool{"configured": true}
			}
		}
	case "/api/model/trial":
		var q struct {
			Baseline    string `json:"baseline"`
			Destination string `json:"destination"`
		}
		if err = readJSON(r, &q); err == nil {
			trialCtx, stop := context.WithTimeout(ctx, 50*time.Second)
			out, err = a.modelTrial(trialCtx, q.Baseline, q.Destination)
			stop()
			if err == nil {
				b, _ := json.Marshal(out)
				_, err = a.db.Exec("INSERT INTO model_runs(time,result) VALUES(?,?)", time.Now().UTC().Format(time.RFC3339), b)
			}
		}
	case "/api/model/results":
		var rows *sql.Rows
		rows, err = a.db.Query("SELECT result FROM model_runs ORDER BY id DESC LIMIT 100")
		if err == nil {
			results := []json.RawMessage{}
			for rows.Next() {
				var b []byte
				if err = rows.Scan(&b); err != nil {
					break
				}
				results = append(results, json.RawMessage(b))
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			out = results
		}
	case "/api/connect":
		var q struct {
			Source string `json:"source"`
			Demo   bool   `json:"demo"`
		}
		if err = readJSON(r, &q); err == nil {
			if q.Demo {
				q.Source, err = a.demo(ctx)
			}
			if err == nil && q.Source == "" {
				err = fmt.Errorf("choose a repository folder")
			}
			if err == nil {
				err = a.connect(ctx, q.Source, false)
			}
			if err == nil {
				out, err = a.status(ctx)
			}
		}
	case "/api/pick":
		var dir string
		dir, err = pickFolder(ctx)
		out = map[string]string{"path": dir}
	case "/api/export":
		if a.service == nil {
			err = fmt.Errorf("connect a workspace first")
			break
		}
		var head string
		head, err = a.service.Head(ctx, "local", "workspace")
		if err != nil {
			break
		}
		var archive []byte
		archive, err = a.service.Archive(ctx, "local", "workspace", head)
		if err != nil {
			break
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="notsofast-snapshot.zip"`)
		_, _ = w.Write(archive)
		return
	case "/api/quit":
		out = map[string]bool{"closing": true}
		if a.Quit != nil {
			defer a.Quit()
		}
	default:
		op := strings.TrimPrefix(r.URL.Path, "/api/call/")
		if !strings.HasPrefix(r.URL.Path, "/api/call/") || a.service == nil {
			err = fmt.Errorf("connect a repository first")
			break
		}
		var body []byte
		body, err = io.ReadAll(r.Body)
		if err != nil {
			break
		}
		if len(body) > 128<<10 {
			err = fmt.Errorf("desktop operation exceeds 128 KiB request limit")
			break
		}
		// Persist creation intent before calling the core. Lost responses remain discoverable by operation ID.
		var eventID int64
		if op == "guarded_create" {
			var q core.CreateRequest
			if err = json.Unmarshal(body, &q); err != nil {
				break
			}
			if len(q.Content) > 65536 {
				err = fmt.Errorf("file exceeds 64 KiB desktop limit")
				break
			}
			var res sql.Result
			res, err = a.db.Exec("INSERT INTO events(source,time,action,state,request,result) VALUES(?,?,?,'PENDING',?,'null')", a.source, time.Now().UTC().Format(time.RFC3339), op, body)
			if err != nil {
				break
			}
			eventID, _ = res.LastInsertId()
		}
		out, err = protocol.Dispatch(ctx, a.service, "local", op, body, false)
		state := "DONE"
		if err != nil {
			state = core.Code(err)
		}
		data, _ := json.Marshal(out)
		if err != nil {
			data, _ = json.Marshal(map[string]string{"reason": core.Code(err), "message": err.Error()})
		}
		// Persist small summaries only. Requests can contain file text and stay in local operator storage.
		if eventID != 0 {
			_, saveErr := a.db.Exec("UPDATE events SET state=?,result=? WHERE id=?", state, data, eventID)
			if saveErr != nil {
				err = fmt.Errorf("operation may have completed; recover by its ID: %w", saveErr)
			}
		} else if op != "head" && op != "snapshot" && len(body) <= 128<<10 {
			_, saveErr := a.db.Exec("INSERT INTO events(source,time,action,state,request,result) VALUES(?,?,?,?,?,?)", a.source, time.Now().UTC().Format(time.RFC3339), op, state, body, data)
			if saveErr != nil && err == nil {
				err = saveErr
			}
		}
	}
	reply(w, out, err)
}
