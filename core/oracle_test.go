package core

import (
	"bytes"
	"os/exec"
	"path"
	"strings"
	"testing"
)

// Deliberately independent: no manifest, cache, domain or derivation helpers.
func oracleTest(t *testing.T, repo, commit string, p Predicate) map[string]string {
	t.Helper()
	b, e := exec.Command("git", "-C", repo, "ls-tree", "-r", "-t", "-z", commit).Output()
	if e != nil {
		t.Fatal(e)
	}
	out := map[string]string{}
	for _, row := range bytes.Split(b, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		parts := bytes.SplitN(row, []byte{'\t'}, 2)
		f := strings.Fields(string(parts[0]))
		name := string(parts[1])
		match := false
		switch p.Kind {
		case "exact_path":
			match = name == string(p.Value)
		case "exact_basename":
			match = path.Base(name) == string(p.Value)
		case "literal_bytes":
			if f[0] != "100644" && f[0] != "100755" {
				continue
			}
			content, e := exec.Command("git", "-C", repo, "cat-file", "blob", f[2]).Output()
			if e != nil {
				t.Fatal(e)
			}
			match = bytes.Contains(content, p.Value)
		}
		out[name] = "NO_MATCH"
		if match {
			out[name] = "MATCH"
		}
	}
	return out
}
