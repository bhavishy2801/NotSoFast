package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const branch = "refs/heads/notsofast"
const maxGitOutput = 64 << 20

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, fmt.Errorf("Git output limit exceeded")
	}
	return b.Buffer.Write(p)
}
func git(ctx context.Context, dir string, input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	base := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.fsmonitor=false", "-c", "gc.auto=0", "-c", "protocol.allow=never", "-c", "core.commitGraph=false", "-C", dir}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	for _, v := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(v, "=", 2)[0])
		if !strings.HasPrefix(k, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_NAME=NotSoFast", "GIT_AUTHOR_EMAIL=local@notsofast.invalid", "GIT_COMMITTER_NAME=NotSoFast", "GIT_COMMITTER_EMAIL=local@notsofast.invalid")
	cmd.Stdin = bytes.NewReader(input)
	out := &limitedBuffer{limit: maxGitOutput}
	errout := &limitedBuffer{limit: 8192}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e := cmd.Run(); e != nil {
		return nil, fail("GIT_ERROR", "Git command failed: "+args[0])
	}
	return out.Bytes(), nil
}
func gitText(ctx context.Context, dir string, input []byte, args ...string) (string, error) {
	b, e := git(ctx, dir, input, args...)
	return strings.TrimSpace(string(b)), e
}
func (s *Service) repo(id string) string {
	return filepath.Join(s.cfg.Root, "repos", digest(id)+".git")
}
func (s *Service) Register(ctx context.Context, user, id, commit string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.authorize(user, id, false); e != nil {
		return Snapshot{}, e
	}
	if e := requireOID(commit); e != nil {
		return Snapshot{}, e
	}
	if e := s.budget(); e != nil {
		return Snapshot{}, e
	}
	source, ok := s.cfg.Repositories[id]
	if !ok {
		return Snapshot{}, fail("FORBIDDEN", "repository unavailable")
	}
	dest := s.repo(id)
	if _, e := os.Stat(dest); e == nil {
		head, err := gitText(ctx, dest, nil, "rev-parse", branch)
		if err == nil {
			if head != commit {
				return Snapshot{}, fail("ALREADY_REGISTERED", "managed repository already initialized")
			}
			return s.snapshot(ctx, user, id, commit)
		}
	}
	format, e := gitText(ctx, source, nil, "rev-parse", "--show-object-format")
	if e != nil {
		return Snapshot{}, e
	}
	if format != "sha1" && format != "sha256" {
		return Snapshot{}, fail("UNSUPPORTED", "object format")
	}
	if _, e = git(ctx, source, nil, "cat-file", "-e", commit+"^{commit}"); e != nil {
		return Snapshot{}, e
	}
	pack, e := git(ctx, source, []byte(commit+"\n"), "pack-objects", "--revs", "--stdout")
	if e != nil {
		return Snapshot{}, e
	}
	if e = os.MkdirAll(dest, 0700); e != nil {
		return Snapshot{}, e
	}
	if _, e = git(ctx, dest, nil, "init", "--bare", "--object-format="+format); e != nil {
		return Snapshot{}, e
	}
	if _, e = git(ctx, dest, pack, "index-pack", "--stdin"); e != nil {
		return Snapshot{}, e
	}
	snap, e := s.manifest(ctx, id, commit)
	if e != nil {
		return Snapshot{}, e
	}
	if e = s.budget(); e != nil {
		return Snapshot{}, e
	}
	if _, e = git(ctx, dest, []byte("create "+branch+" "+commit+"\n"), "update-ref", "--stdin"); e != nil {
		return Snapshot{}, e
	}
	return snap, nil
}
func (s *Service) Head(ctx context.Context, user, id string) (string, error) {
	if e := s.authorize(user, id, false); e != nil {
		return "", e
	}
	return gitText(ctx, s.repo(id), nil, "rev-parse", "--verify", branch)
}
func (s *Service) Snapshot(ctx context.Context, user, id, commit string) (Snapshot, error) {
	return s.snapshot(ctx, user, id, commit)
}
func (s *Service) snapshot(ctx context.Context, user, id, commit string) (Snapshot, error) {
	if e := s.authorize(user, id, false); e != nil {
		return Snapshot{}, e
	}
	if e := requireOID(commit); e != nil {
		return Snapshot{}, e
	}
	if _, e := git(ctx, s.repo(id), nil, "merge-base", "--is-ancestor", commit, branch); e != nil {
		return Snapshot{}, fail("BAD_SNAPSHOT", "snapshot is not in managed history")
	}
	return s.manifest(ctx, id, commit)
}
func (s *Service) manifest(ctx context.Context, id, commit string) (Snapshot, error) {
	snap := Snapshot{Repository: id, Commit: commit, Version: 1}
	dir := s.repo(id)
	var e error
	snap.Tree, e = gitText(ctx, dir, nil, "rev-parse", commit+"^{tree}")
	if e != nil {
		return snap, e
	}
	snap.Algorithm, e = gitText(ctx, dir, nil, "rev-parse", "--show-object-format")
	if e != nil {
		return snap, e
	}
	b, e := git(ctx, dir, nil, "ls-tree", "-r", "-t", "-z", "--full-tree", commit)
	if e != nil {
		return snap, e
	}
	for _, row := range bytes.Split(b, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(parts) != 2 || !utf8.Valid(parts[1]) {
			return snap, fail("MANIFEST_ERROR", "unsupported path encoding")
		}
		fields := strings.Fields(string(parts[0]))
		if len(fields) != 3 || !oidValid(fields[2]) {
			return snap, fail("MANIFEST_ERROR", "invalid tree entry")
		}
		ent := Entry{Path: string(parts[1]), Mode: fields[0], OID: fields[2]}
		if !validPath(ent.Path) {
			return snap, fail("MANIFEST_ERROR", "unsupported path")
		}
		switch ent.Mode {
		case "100644", "100755":
			ent.Type = "regular"
		case "120000":
			ent.Type = "symlink"
		case "160000":
			ent.Type = "gitlink"
		case "040000":
			ent.Type = "tree"
		default:
			return snap, fail("MANIFEST_ERROR", "unsupported mode")
		}
		snap.Entries = append(snap.Entries, ent)
		if len(snap.Entries) > s.cfg.MaxEntries {
			return snap, fail("LIMIT", "manifest entry limit")
		}
	}
	// Presence is checked even for name predicates. Gitlinks refer outside this universe.
	var check strings.Builder
	for _, ent := range snap.Entries {
		if ent.Type != "gitlink" {
			check.WriteString(ent.OID + "\n")
		}
	}
	b, e = git(ctx, dir, []byte(check.String()), "cat-file", "--batch-check=%(objectname) %(objecttype)")
	if e != nil {
		return snap, e
	}
	if bytes.Contains(b, []byte(" missing")) {
		return snap, fail("MANIFEST_ERROR", "missing Git object")
	}
	sort.Slice(snap.Entries, func(i, j int) bool { return snap.Entries[i].Path < snap.Entries[j].Path })
	snap.Manifest = digest(struct {
		Version int
		Entries []Entry
	}{1, snap.Entries})
	return snap, nil
}
func (s *Service) blob(ctx context.Context, repo, oid string) ([]byte, error) {
	n, e := gitText(ctx, s.repo(repo), nil, "cat-file", "-s", oid)
	if e != nil {
		return nil, fail("UNREADABLE", "object unavailable")
	}
	var size int64
	if _, e = fmt.Sscan(n, &size); e != nil {
		return nil, e
	}
	if size > s.cfg.MaxBlobBytes {
		return nil, fail("OVERSIZED", "blob exceeds evaluation limit")
	}
	return git(ctx, s.repo(repo), nil, "cat-file", "blob", oid)
}
func dirSize(root string) (int64, error) {
	var n int64
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			i, e := d.Info()
			if e != nil {
				return e
			}
			n += i.Size()
		}
		return nil
	})
	return n, e
}

var _ io.Writer = (*limitedBuffer)(nil)
