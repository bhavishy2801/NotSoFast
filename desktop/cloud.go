package desktop

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type CloudConfig struct {
	URL string `json:"url"`
	Key string `json:"key"`
}
type cloudSession struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Expires int64  `json:"expires_in"`
	User    struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}
type cloudState struct {
	Config         CloudConfig
	Session        cloudSession
	Until          time.Time
	Verifier, Flow string
	Started        time.Time
}

var projectHost = regexp.MustCompile(`^[a-z0-9-]+\.supabase\.co$`)

func (c CloudConfig) validate() error {
	u, e := url.Parse(c.URL)
	if e != nil || u.Scheme != "https" || !projectHost.MatchString(u.Host) || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("enter your HTTPS Supabase project URL")
	}
	if len(c.Key) > 4096 {
		return fmt.Errorf("invalid public key")
	}
	if strings.HasPrefix(c.Key, "sb_publishable_") && len(c.Key) > 20 {
		return nil
	}
	parts := strings.Split(c.Key, ".")
	if len(parts) == 3 {
		b, e := base64.RawURLEncoding.DecodeString(parts[1])
		var claims struct {
			Role string `json:"role"`
		}
		if e == nil && json.Unmarshal(b, &claims) == nil && claims.Role == "anon" {
			return nil
		}
	}
	return fmt.Errorf("use a publishable or legacy anon key, never a secret or service-role key")
}
func randomURLToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *App) cloudView() any {
	return map[string]any{"configured": a.cloud.Config.URL != "", "url": a.cloud.Config.URL, "signed_in": a.cloud.Session.Access != "", "email": a.cloud.Session.User.Email, "pending": a.cloud.Flow != "" && time.Since(a.cloud.Started) < 10*time.Minute}
}
func (a *App) cloudRequest(ctx context.Context, method, path string, body, out any, auth bool) error {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(a.cloud.Config.URL, "/")+path, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("apikey", a.cloud.Config.Key)
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+a.cloud.Session.Access)
	}
	client := modelHTTP()
	defer client.CloseIdleConnections()
	res, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("cloud service unavailable; your local work is saved")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 1<<20 {
		return fmt.Errorf("cloud response too large")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("cloud returned HTTP %d; check project setup, sign-in and database policies", res.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
func (a *App) cloudAuth(ctx context.Context) error {
	if a.cloud.Session.Access == "" {
		return fmt.Errorf("sign in to your cloud account first")
	}
	if time.Now().Before(a.cloud.Until.Add(-time.Minute)) {
		return nil
	}
	var next cloudSession
	if e := a.cloudRequest(ctx, "POST", "/auth/v1/token?grant_type=refresh_token", map[string]string{"refresh_token": a.cloud.Session.Refresh}, &next, false); e != nil {
		return e
	}
	if next.Access == "" || next.User.ID != a.cloud.Session.User.ID {
		return fmt.Errorf("cloud session could not be refreshed")
	}
	a.cloud.Session = next
	a.cloud.Until = time.Now().Add(time.Duration(next.Expires) * time.Second)
	return nil
}
func (a *App) cloudStart(provider string) (any, error) {
	if e := a.cloud.Config.validate(); e != nil {
		return nil, e
	}
	if provider != "github" && provider != "google" {
		return nil, fmt.Errorf("unsupported sign-in provider")
	}
	a.cloud.Verifier = randomURLToken()
	a.cloud.Flow = randomURLToken()
	a.cloud.Started = time.Now()
	sum := sha256.Sum256([]byte(a.cloud.Verifier))
	q := url.Values{"provider": {provider}, "redirect_to": {"http://" + a.Host + "/auth/callback?flow=" + a.cloud.Flow}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"s256"}}
	if err := OpenBrowser(strings.TrimRight(a.cloud.Config.URL, "/") + "/auth/v1/authorize?" + q.Encode()); err != nil {
		a.cloud.Flow = ""
		return nil, err
	}
	return a.cloudView(), nil
}
func (a *App) cloudCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" || !a.mu.TryLock() {
		http.Error(w, "App busy. Retry this page after the current operation finishes.", 409)
		return
	}
	defer a.mu.Unlock()
	flow := r.URL.Query().Get("flow")
	if a.cloud.Flow == "" || subtle.ConstantTimeCompare([]byte(flow), []byte(a.cloud.Flow)) != 1 || time.Since(a.cloud.Started) > 10*time.Minute {
		http.Error(w, "Sign-in expired or did not start in this app. Start sign-in again.", 400)
		return
	}
	verifier := a.cloud.Verifier
	a.cloud.Flow = ""
	a.cloud.Verifier = ""
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 2048 {
		http.Error(w, "Sign-in cancelled. Return to the app and try again.", 400)
		return
	}
	var session cloudSession
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	err := a.cloudRequest(ctx, "POST", "/auth/v1/token?grant_type=pkce", map[string]string{"auth_code": code, "code_verifier": verifier}, &session, false)
	if err != nil || session.Access == "" || session.User.ID == "" {
		http.Error(w, "Could not complete sign-in. Check the cloud configuration and start again.", 400)
		return
	}
	a.cloud.Session = session
	a.cloud.Until = time.Now().Add(time.Duration(session.Expires) * time.Second)
	http.Redirect(w, r, "/#"+a.token, http.StatusSeeOther)
}

type cloudSnapshot struct {
	ID      string  `json:"id,omitempty"`
	UserID  string  `json:"user_id"`
	Created string  `json:"created_at,omitempty"`
	Profile Profile `json:"payload"`
}

func (a *App) cloudSnapshots(ctx context.Context, save bool) (any, error) {
	if e := a.cloudAuth(ctx); e != nil {
		return nil, e
	}
	if save {
		p := a.profile()
		if e := p.validate(); e != nil {
			return nil, e
		}
		body := map[string]any{"user_id": a.cloud.Session.User.ID, "payload": p}
		if e := a.cloudRequest(ctx, "POST", "/rest/v1/nsf_snapshots", body, nil, true); e != nil {
			return nil, e
		}
	}
	var snapshots []cloudSnapshot
	path := "/rest/v1/nsf_snapshots?select=id,user_id,created_at,payload&user_id=eq." + url.QueryEscape(a.cloud.Session.User.ID) + "&order=created_at.desc&limit=10"
	if e := a.cloudRequest(ctx, "GET", path, nil, &snapshots, true); e != nil {
		return nil, e
	}
	for _, s := range snapshots {
		if s.UserID != a.cloud.Session.User.ID {
			return nil, fmt.Errorf("cloud ownership mismatch")
		}
		if e := s.Profile.validate(); e != nil {
			return nil, e
		}
	}
	return snapshots, nil
}
