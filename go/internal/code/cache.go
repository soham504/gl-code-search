package code

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
)

// cacheVersion must change whenever Analyze, parseSymbols or the title
// layout in BuildCached changes, so stale term frequencies are never reused.
const cacheVersion = 2

type cacheEntry struct {
	Size, ModTime int64
	TF            map[string]int
	Len           int
	Symbols       []Symbol
}

// Cache holds per-file analysis results for one source tree, persisted under
// the user cache directory. Methods are nil-safe: a nil *Cache caches nothing.
type Cache struct {
	Version int
	Entries map[string]cacheEntry
	path    string
	dirty   bool
}

// OpenCache loads (or starts) the cache for root. Errors only disable caching.
func OpenCache(root string) *Cache {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	sum := sha256.Sum256([]byte(abs))
	c := &Cache{Version: cacheVersion, Entries: map[string]cacheEntry{},
		path: filepath.Join(dir, "groundline", hex.EncodeToString(sum[:8])+".gob")}
	if f, err := os.Open(c.path); err == nil {
		var old Cache
		if gob.NewDecoder(f).Decode(&old) == nil && old.Version == cacheVersion {
			c.Entries = old.Entries
		}
		f.Close()
	}
	return c
}

func (c *Cache) lookup(f *File) (cacheEntry, bool) {
	if c == nil || f.ModTime == 0 {
		return cacheEntry{}, false
	}
	e, ok := c.Entries[f.Path]
	return e, ok && e.Size == f.Size && e.ModTime == f.ModTime
}

func (c *Cache) store(f *File, e cacheEntry) {
	if c == nil || f.ModTime == 0 {
		return
	}
	e.Size, e.ModTime = f.Size, f.ModTime
	c.Entries[f.Path] = e
	c.dirty = true
}

// prune drops entries for files that no longer exist.
func (c *Cache) prune(files []File) {
	if c == nil || len(c.Entries) == len(files) {
		return
	}
	live := make(map[string]bool, len(files))
	for _, f := range files {
		live[f.Path] = true
	}
	for p := range c.Entries {
		if !live[p] {
			delete(c.Entries, p)
			c.dirty = true
		}
	}
}

// Save writes the cache if anything changed. It writes to a temp file and
// renames, so concurrent gl processes never see a torn cache.
func (c *Cache) Save() error {
	if c == nil || !c.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), "cache-*.gob")
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(tmp).Encode(c); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	c.dirty = false
	return os.Rename(tmp.Name(), c.path)
}
