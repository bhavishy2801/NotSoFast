package desktop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var githubSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
var deviceCode = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)

func parseGitHubURL(input string) (string, string, error) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "github.com/") {
		input = "https://" + input
	}
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("use a GitHub repository URL such as https://github.com/owner/repository")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("paste the repository URL, without a branch or file path")
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, part := range parts {
		if !githubSegment.MatchString(part) || part == "." || part == ".." {
			return "", "", fmt.Errorf("invalid GitHub repository name")
		}
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), nil
}

type githubView struct {
	Available bool   `json:"available"`
	SignedIn  bool   `json:"signed_in"`
	Login     string `json:"login"`
	Stage     string `json:"stage"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	URL       string `json:"verification_url,omitempty"`
}
type gitHubConnection struct {
	mu         sync.Mutex
	root, exe  string
	view       githubView
	cancel     context.CancelFunc
	done       chan struct{}
	authOutput string
}

func newGitHub(root string) *gitHubConnection {
	exe, _ := exec.LookPath("gh")
	g := &gitHubConnection{root: filepath.Join(root, "github-auth"), exe: exe}
	g.view = githubView{Available: exe != "", Stage: "disconnected"}
	return g
}
func (g *gitHubConnection) env() []string {
	var env []string
	for _, v := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(v, "=", 2)[0])
		if !strings.HasPrefix(k, "GH_") && !strings.HasPrefix(k, "GITHUB_") && !strings.HasPrefix(k, "GIT_") {
			env = append(env, v)
		}
	}
	return append(env, "GH_CONFIG_DIR="+g.root, "GH_HOST=github.com", "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
}
func (g *gitHubConnection) command(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, g.exe, args...)
	quiet(c)
	c.Env = g.env()
	return c
}

type cappedOutput struct {
	bytes.Buffer
	max int
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	if len(p) > b.max-b.Len() {
		return 0, fmt.Errorf("command output limit reached")
	}
	return b.Buffer.Write(p)
}
func (g *gitHubConnection) output(ctx context.Context, args ...string) (string, error) {
	if g.exe == "" {
		return "", fmt.Errorf("GitHub support is missing; use the complete portable package")
	}
	c := g.command(ctx, args...)
	b := &cappedOutput{max: 4096}
	c.Stdout = b
	c.Stderr = io.Discard
	if err := c.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}
func (g *gitHubConnection) status() githubView { g.mu.Lock(); defer g.mu.Unlock(); return g.view }

// Write receives GitHub CLI's device-flow prompt; never forwards raw subprocess output to the page.
func (g *gitHubConnection) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.authOutput) < 8192 {
		g.authOutput += string(p)
		if code := deviceCode.FindString(g.authOutput); code != "" {
			g.view.Code = code
			g.view.URL = "https://github.com/login/device"
			g.view.Stage = "authorizing"
			g.view.Message = "Enter this one-time code in the GitHub browser window."
		}
	}
	return len(p), nil
}
func (g *gitHubConnection) refresh(ctx context.Context) githubView {
	g.mu.Lock()
	running := g.cancel != nil
	g.mu.Unlock()
	if running || g.exe == "" {
		return g.status()
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	login, err := g.output(ctx, "api", "user", "--jq", ".login")
	g.mu.Lock()
	defer g.mu.Unlock()
	if err == nil && login != "" {
		g.view.SignedIn = true
		g.view.Login = login
		g.view.Stage = "connected"
	} else {
		g.view.SignedIn = false
		g.view.Login = ""
		g.view.Stage = "disconnected"
	}
	return g.view
}
func (g *gitHubConnection) start() (githubView, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.exe == "" {
		return g.view, fmt.Errorf("GitHub support is missing; use the complete portable package")
	}
	if g.cancel != nil {
		return g.view, nil
	}
	if err := os.MkdirAll(g.root, 0700); err != nil {
		return g.view, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	g.cancel = cancel
	g.done = make(chan struct{})
	g.authOutput = ""
	g.view = githubView{Available: true, Stage: "starting", Message: "Requesting a one-time sign-in code from GitHub…"}
	done := g.done
	go func() {
		defer close(done)
		defer cancel()
		c := g.command(ctx, "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web", "--skip-ssh-key")
		c.Stdin = strings.NewReader("\n")
		c.Stdout = g
		c.Stderr = g
		err := c.Run()
		var login string
		if err == nil {
			login, err = g.output(ctx, "api", "user", "--jq", ".login")
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		g.cancel = nil
		g.view.Code = ""
		g.view.URL = ""
		g.authOutput = ""
		if err != nil {
			g.view.Stage = "error"
			g.view.Message = "Sign-in was cancelled, expired, or could not reach GitHub. Try again."
		} else {
			g.view.Stage = "connected"
			g.view.SignedIn = true
			g.view.Login = login
			g.view.Message = "GitHub connected."
		}
	}()
	return g.view, nil
}
func (g *gitHubConnection) close() {
	g.mu.Lock()
	cancel, done := g.cancel, g.done
	g.mu.Unlock()
	if cancel != nil {
		cancel()
		if done != nil {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	}
}
func (g *gitHubConnection) disconnect() error {
	g.close()
	g.mu.Lock()
	defer g.mu.Unlock()
	// Remove only this application's account configuration. Do not revoke or erase credentials used by other GitHub CLI installations.
	if err := os.Remove(filepath.Join(g.root, "hosts.yml")); err != nil && !os.IsNotExist(err) {
		return err
	}
	g.view = githubView{Available: g.exe != "", Stage: "disconnected"}
	return nil
}
func (a *App) importGitHub(ctx context.Context, input string) (string, error) {
	owner, repo, err := parseGitHubURL(input)
	if err != nil {
		return "", err
	}
	canonical := "https://github.com/" + owner + "/" + repo + ".git"
	imports := filepath.Join(a.Root, "github-repositories")
	if err = os.MkdirAll(imports, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(imports, "incoming-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	token, _ := a.github.output(ctx, "auth", "token", "--hostname", "github.com")
	args := []string{"-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "http.followRedirects=false", "clone", "--bare", "--single-branch", "--no-tags", "--", canonical, temp}
	c := exec.CommandContext(ctx, "git", args...)
	quiet(c)
	cancelTree(c)
	for _, v := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(v, "=", 2)[0])
		if !strings.HasPrefix(k, "GIT_") && !strings.HasPrefix(k, "GCM_") {
			c.Env = append(c.Env, v)
		}
	}
	c.Env = append(c.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "GIT_LFS_SKIP_SMUDGE=1")
	if token != "" {
		c.Env = append(c.Env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token)))
	}
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				var size int64
				_ = filepath.WalkDir(temp, func(_ string, d os.DirEntry, e error) error {
					if e == nil && !d.IsDir() {
						if info, e := d.Info(); e == nil {
							size += info.Size()
						}
					}
					if size > 96<<20 {
						return fmt.Errorf("limit")
					}
					return nil
				})
				if size > 96<<20 {
					cancel()
					return
				}
			}
		}
	}()
	if err = c.Run(); err != nil {
		return "", fmt.Errorf("GitHub import failed. Check the URL, sign in for private repositories, and ensure the repository fits the 64 MiB import limit; network imports time out after 50 seconds")
	}
	head, err := localGit(ctx, temp, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(canonical + "@" + head))
	dest := filepath.Join(imports, hex.EncodeToString(key[:16]))
	if _, err = os.Stat(dest); err == nil {
		return dest, nil
	}
	if err = os.Rename(temp, dest); err != nil {
		return "", err
	}
	return dest, nil
}
