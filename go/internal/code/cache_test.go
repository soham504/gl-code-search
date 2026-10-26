package code

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheReuseAndInvalidate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // os.UserCacheDir on macOS
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	write := func(name, src string) {
		p := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pkg/a.py", "def alpha(): pass\n")
	write("pkg/b.py", "def beta(): pass\n")

	build := func() *Index {
		files, err := LoadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		c := OpenCache(root)
		ix := BuildCached(files, c)
		if err := c.Save(); err != nil {
			t.Fatal(err)
		}
		return ix
	}
	build()
	if c := OpenCache(root); len(c.Entries) != 2 {
		t.Fatalf("cache has %d entries, want 2", len(c.Entries))
	}

	// Editing a file must invalidate its entry: the new symbol must be found.
	time.Sleep(10 * time.Millisecond)
	write("pkg/a.py", "def gamma(): pass\n")
	ix := build()
	if got := ix.Definitions("gamma"); len(got) != 1 {
		t.Errorf("edited file not re-analyzed: Definitions(gamma) = %q", got)
	}
	if got := ix.Definitions("alpha"); len(got) != 0 {
		t.Errorf("stale symbol served from cache: %q", got)
	}

	// Deleting a file must prune its entry.
	os.Remove(filepath.Join(root, "pkg/b.py"))
	build()
	if c := OpenCache(root); len(c.Entries) != 1 {
		t.Errorf("deleted file not pruned: %d entries", len(c.Entries))
	}
}

func TestNilCacheIsSafe(t *testing.T) {
	var c *Cache
	ix := BuildCached(corpus(), c)
	if ix.Len() != 4 || c.Save() != nil {
		t.Fatal("nil cache should be a no-op")
	}
}
