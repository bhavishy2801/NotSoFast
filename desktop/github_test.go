package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubURLBoundary(t *testing.T) {
	for _, input := range []string{"https://github.com/owner/repo", "https://github.com/Owner/repo.git/", "github.com/owner/repo"} {
		owner, repo, err := parseGitHubURL(input)
		if err != nil || owner != "owner" || repo != "repo" {
			t.Fatalf("%s: %s %s %v", input, owner, repo, err)
		}
	}
	for _, input := range []string{"https://evil.test/owner/repo", "https://github.com@evil.test/a/b", "https://github.com/a/..", "https://github.com/a/b/tree/main", "https://github.com/a/b?token=secret", "file:///a/b", "https://github.com:443/a/b", "https://user@github.com/a/b", "https://github.com/a/%2e%2e"} {
		if _, _, err := parseGitHubURL(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

func TestGitHubPublicImport(t *testing.T) {
	if os.Getenv("NSF_GITHUB_INTEGRATION") != "1" {
		t.Skip("explicit network integration test")
	}
	a, err := New(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	source, err := a.importGitHub(context.Background(), "https://github.com/octocat/Hello-World")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.connect(context.Background(), source, false); err != nil {
		t.Fatal(err)
	}
	managed, err := filepath.Glob(filepath.Join(a.cfg.Root, "repos", "*.git"))
	if err != nil || len(managed) != 1 {
		t.Fatalf("expected one managed repository: %v, %v", managed, err)
	}
	if _, err = localGit(context.Background(), managed[0], "fsck", "--full"); err != nil {
		t.Fatalf("imported managed history is incomplete: %v", err)
	}
	head, err := a.service.Head(context.Background(), "local", "workspace")
	if err != nil {
		t.Fatal(err)
	}
	data, err := a.service.ReadFile(context.Background(), "local", "workspace", head, "README")
	if err != nil || len(data) == 0 {
		t.Fatalf("preview %s %v", data, err)
	}
	if _, err = a.service.ReadFile(context.Background(), "local", "workspace", head, "../README"); err == nil {
		t.Fatal("parent path accepted")
	}
}

func TestGitHubPromptBoundary(t *testing.T) {
	g := &gitHubConnection{}
	g.Write([]byte("First copy your one-time code: ABCD-"))
	g.Write([]byte("1234\nraw text must not reach UI"))
	v := g.status()
	if v.Code != "ABCD-1234" || v.URL != "https://github.com/login/device" || v.Stage != "authorizing" {
		t.Fatalf("bad prompt: %+v", v)
	}
	b := &cappedOutput{max: 4}
	if _, err := b.Write([]byte("12345")); err == nil || b.Len() != 0 {
		t.Fatal("output bound failed")
	}
}
