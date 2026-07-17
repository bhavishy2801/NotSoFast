package desktop

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopWorkflowAndBoundary(t *testing.T) {
	a, err := New(t.TempDir(), "secret-session")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Host = "127.0.0.1:9999"
	call := func(route string, body any, token, origin string) (int, map[string]any) {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "http://"+a.Host+route, bytes.NewReader(b))
		r.Header.Set("X-NSF-Session", token)
		r.Header.Set("X-NSF-Workspace", a.workspaceID())
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, _ := call("/api/status", nil, "", ""); code != 401 {
		t.Fatalf("unauthenticated %d", code)
	}
	if code, _ := call("/api/status", nil, "secret-session", "https://evil.test"); code != 403 {
		t.Fatalf("cross origin %d", code)
	}
	if code, _ := call("/api/connect", map[string]any{"demo": true, "unknown": true}, "secret-session", ""); code != 400 {
		t.Fatalf("unknown fields %d", code)
	}
	code, out := call("/api/connect", map[string]bool{"demo": true}, "secret-session", "")
	if code != 200 {
		t.Fatalf("connect %d %v", code, out)
	}
	state := out["result"].(map[string]any)
	snap := state["snapshot"].(map[string]any)["commit"].(string)
	q := map[string]any{"repository": "workspace", "snapshot": snap, "predicate": map[string]any{"kind": "exact_basename", "version": 1, "value": base64.StdEncoding.EncodeToString([]byte("fresh.txt"))}, "scope": map[string]any{}}
	code, out = call("/api/call/search", q, "secret-session", "")
	if code != 200 {
		t.Fatal(out)
	}
	rid := out["result"].(map[string]any)["receipt"].(map[string]any)["id"].(string)
	create := map[string]any{"repository": "workspace", "snapshot": snap, "operation": "desktop-test", "policy": "unique", "policy_version": "1", "path": "fresh.txt", "content": "b2s=", "receipts": []string{rid}}
	code, out = call("/api/call/guarded_create", create, "secret-session", "")
	if code != 200 {
		t.Fatal(out)
	}
	candidate := out["result"].(map[string]any)["candidate"]
	code, out = call("/api/call/guarded_create", create, "secret-session", "")
	if code != 200 || out["result"].(map[string]any)["candidate"] != candidate {
		t.Fatalf("retry %v", out)
	}
	code, out = call("/api/status", nil, "secret-session", "")
	if code != 200 {
		t.Fatal(out)
	}
	if len(out["result"].(map[string]any)["events"].([]any)) < 3 {
		t.Fatal("missing durable events")
	}
	root := a.Root
	a.Close()
	a, err = New(root, "secret-session")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Host = "127.0.0.1:9999"
	code, out = call("/api/status", nil, "secret-session", "")
	if code != 200 || out["result"].(map[string]any)["snapshot"].(map[string]any)["commit"] != candidate {
		t.Fatalf("restart %v", out)
	}
}

func TestWorkspaceBindingAndIncompleteSample(t *testing.T) {
	a, err := New(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.Host = "127.0.0.1:9999"
	dir := filepath.Join(a.Root, "sample-repository")
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = localGit(context.Background(), dir, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.demo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = a.connect(context.Background(), dir, false); err != nil {
		t.Fatalf("incomplete sample not repaired: %v", err)
	}
	r := httptest.NewRequest("POST", "http://"+a.Host+"/api/call/head", bytes.NewBufferString(`{"repository":"workspace"}`))
	r.Header.Set("X-NSF-Session", "test")
	r.Header.Set("X-NSF-Workspace", "stale-workspace")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("stale workspace accepted: %d %s", w.Code, w.Body.String())
	}
}
