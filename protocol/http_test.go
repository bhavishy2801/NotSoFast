package protocol

import (
	"net/http"
	"net/http/httptest"
	"notsofast/core"
	"strings"
	"testing"
)

func TestHTTPAuthenticationAndBounds(t *testing.T) {
	token := "test-only-token-at-least-24-bytes"
	s, e := core.Open(core.Config{Root: t.TempDir(), Principals: map[string]core.Principal{"alice": {}}, Tokens: map[string]string{token: "alice"}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := NewHandler(s, 1)
	for _, tc := range []struct {
		token, path, body, origin string
		want                      int
	}{{"", "/v1/head", "{}", "", 401}, {token, "/v1/head", "{}", "https://evil.example", 403}, {token, "/v1/head", `{"repository":"private"}`, "", 403}, {token, "/v1/search", strings.Repeat(" ", 2<<20) + "{}", "", 413}, {token, "/metrics", "", "", 200}} {
		method := "POST"
		if tc.path == "/metrics" {
			method = "GET"
		}
		req := httptest.NewRequest(method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	h.slots <- struct{}{}
	defer func() { <-h.slots }()
	req := httptest.NewRequest(http.MethodPost, "/v1/head", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
}
