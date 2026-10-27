package code

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		p := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func paths(files []File) []string {
	var out []string
	for _, f := range files {
		out = append(out, f.Path)
	}
	slices.Sort(out)
	return out
}

func TestLoadDirHonoursGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".gitignore":                  "venv/\ngenerated.py\n",
		"app/core.py":                 "def core(): pass\n",
		"app/new_untracked.py":        "def fresh(): pass\n",
		"generated.py":                "x = 1\n",
		"venv/lib/site-packages/x.py": "def vendored(): pass\n",
	})
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	exec.Command("git", "-C", root, "add", "app/core.py", ".gitignore").Run()
	files, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := paths(files), []string{"app/core.py", "app/new_untracked.py"}; !slices.Equal(got, want) {
		t.Errorf("LoadDir = %q, want %q (ignored files must be skipped, untracked kept)", got, want)
	}
}

func TestLoadDirWalkSkipsJunkAndOversized(t *testing.T) {
	root := t.TempDir() // not a git repo: exercises the walk fallback
	writeTree(t, root, map[string]string{
		"src/main.go":               "package main\nfunc main() {}\n",
		"node_modules/dep/index.js": "module.exports = 1\n",
		"build/out.js":              "var a = 1\n",
		"src/huge.go":               strings.Repeat("// pad\n", MaxFileSize/7+1),
		"src/locked.go":             "package main\n",
	})
	os.Chmod(filepath.Join(root, "src/locked.go"), 0o000)
	defer os.Chmod(filepath.Join(root, "src/locked.go"), 0o644)

	files, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	got := paths(files)
	if slices.Contains(got, "node_modules/dep/index.js") || slices.Contains(got, "build/out.js") ||
		slices.Contains(got, "src/huge.go") {
		t.Errorf("junk or oversized file indexed: %q", got)
	}
	// An unreadable file must not abort indexing: building still succeeds.
	ix := Build(files)
	if len(ix.Definitions("main")) != 1 {
		t.Errorf("readable file lost alongside an unreadable one: %q", got)
	}
}

func TestCachedFilesAreNotRead(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	writeTree(t, root, map[string]string{"a.py": "def alpha(): pass\n", "b.py": "def beta(): pass\n"})
	files, _ := LoadDir(root)
	c := OpenCache(root)
	BuildCached(files, c)
	c.Save()

	files, _ = LoadDir(root)
	ix := BuildCached(files, OpenCache(root))
	for _, f := range ix.files {
		if f.Content != "" {
			t.Errorf("%s was read despite a cache hit", f.Path)
		}
	}
	if len(ix.Definitions("beta")) != 1 {
		t.Error("symbols not restored from cache")
	}
	if s := ix.Snippet("a.py", "alpha", 1); len(s) != 1 {
		t.Errorf("Snippet must load content lazily, got %q", s)
	}
}
