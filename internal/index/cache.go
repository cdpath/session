package index

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cdpath/session/internal/agent"
)

// schemaVersion must be bumped whenever any provider's parsing changes, so
// caches written by older builds are discarded.
const schemaVersion = 1

type cacheFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Entries       map[string]cacheEntry `json:"entries"`
}

type cacheEntry struct {
	Size    int64          `json:"size"`
	ModTime int64          `json:"modTime"`
	Session *agent.Session `json:"session"` // nil: the file is not a session
}

// DefaultCachePath is the per-user cache location.
func DefaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "session", "index.json")
}

func loadCache(path string) map[string]cacheEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]cacheEntry{}
	}
	var c cacheFile
	if json.Unmarshal(data, &c) != nil || c.SchemaVersion != schemaVersion || c.Entries == nil {
		return map[string]cacheEntry{}
	}
	return c.Entries
}

func saveCache(path string, entries map[string]cacheEntry) error {
	data, err := json.Marshal(cacheFile{SchemaVersion: schemaVersion, Entries: entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".index-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// cached returns a private copy of the stored parse result, so derived fields
// set later never leak back into the cache.
func (e cacheEntry) cached() *agent.Session {
	if e.Session == nil {
		return nil
	}
	c := *e.Session
	return &c
}

func newEntry(info os.FileInfo, s *agent.Session) cacheEntry {
	e := cacheEntry{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
	if s != nil {
		c := *s
		e.Session = &c
	}
	return e
}

func (e cacheEntry) fresh(info os.FileInfo) bool {
	return e.Size == info.Size() && e.ModTime == info.ModTime().UnixNano()
}
