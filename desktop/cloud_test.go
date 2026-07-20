package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProfileAndCloudBoundaries(t *testing.T) {
	root := t.TempDir()
	a, e := New(root, "session")
	if e != nil {
		t.Fatal(e)
	}
	p := Profile{Name: "Test", Mode: "dark", Scheme: "violet", DraftPath: "notes.md", Draft: "saved", Bookmarks: []string{"https://github.com/octocat/Hello-World"}}
	if e = a.saveProfile(p); e != nil {
		t.Fatal(e)
	}
	a.Close()
	a, e = New(root, "session")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if a.profile().Draft != "saved" || a.profile().Mode != "dark" {
		t.Fatal("profile did not survive restart")
	}
	for _, cfg := range []CloudConfig{{"http://project.supabase.co", "sb_publishable_testing12345"}, {"https://evil.test", "sb_publishable_testing12345"}, {"https://project.supabase.co", "sb_secret_forbidden"}, {"https://project.supabase.co", "eyJhbGciOiJIUzI1NiJ9.eyJyb2xlIjoic2VydmljZV9yb2xlIn0.signature"}} {
		if cfg.validate() == nil {
			t.Fatal("unsafe configuration accepted")
		}
	}
	if (CloudConfig{"https://project.supabase.co", "sb_publishable_testing12345"}).validate() != nil {
		t.Fatal("public configuration rejected")
	}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/auth/v1/token" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["auth_code"] != "code" || body["code_verifier"] != "verifier" {
				t.Error("PKCE not bound")
			}
			w.Write([]byte(`{"access_token":"private-access","refresh_token":"private-refresh","expires_in":3600,"user":{"id":"user-one","email":"test@example.test"}}`))
			return
		}
		w.Write([]byte(`[{"id":"save","user_id":"another-user","payload":{"mode":"dark","scheme":"lime"}}]`))
	}))
	defer server.Close()
	a.cloud.Config = CloudConfig{server.URL, "public"}
	a.cloud.Flow = "flow"
	a.cloud.Verifier = "verifier"
	a.cloud.Started = time.Now()
	bad := httptest.NewRecorder()
	a.cloudCallback(bad, httptest.NewRequest("GET", "http://localhost/auth/callback?flow=wrong&code=code", nil))
	if bad.Code != 400 || calls != 0 {
		t.Fatal("invalid callback reached token endpoint")
	}
	good := httptest.NewRecorder()
	a.cloudCallback(good, httptest.NewRequest("GET", "http://localhost/auth/callback?flow=flow&code=code", nil))
	if good.Code != 303 || a.cloud.Session.User.ID != "user-one" {
		t.Fatalf("callback failed: %d", good.Code)
	}
	b, _ := json.Marshal(a.cloudView())
	if strings.Contains(string(b), "private-") {
		t.Fatal("credential leaked in status")
	}
	replay := httptest.NewRecorder()
	a.cloudCallback(replay, httptest.NewRequest("GET", "http://localhost/auth/callback?flow=flow&code=code", nil))
	if replay.Code != 400 || calls != 1 {
		t.Fatal("callback replay accepted")
	}
	if _, e = a.cloudSnapshots(context.Background(), false); e == nil {
		t.Fatal("foreign cloud save accepted")
	}
	_, e = a.db.Exec(`INSERT INTO events(source,action,state,request,result) VALUES('repo','guarded_create','DONE','{}','{"outcome":"PUBLISHED","id":"one"}'),('repo','operation','DONE','{}','{"outcome":"PUBLISHED","id":"one"}')`)
	if e != nil { t.Fatal(e) }
	insights, e := a.insights()
	if e != nil {
		t.Fatal(e)
	}
	if insights.(map[string]int)["published"] != 1 { t.Fatal("publication/recovery count should be deduplicated") }
}
