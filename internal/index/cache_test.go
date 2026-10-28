package index_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdpath/session/internal/index"
)

// rewriteSameSize replaces old with new (same length) in a file and restores
// its mtime, so only a content-reading parser could notice the change.
func (f *fixture) rewriteSameSize(path, old, new string, hour int) {
	f.t.Helper()
	if len(old) != len(new) {
		f.t.Fatal("replacement must keep the file size")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.touch(path, hour)
}

func TestCacheReusesUnchangedFiles(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	path := f.droidSession(proj, "d1", 1, obj{"title": "Alpha"}, droidUser("q"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")

	assertList(t, f.list(opts), `droid d1 "Alpha" turns=1`+"\n")

	f.rewriteSameSize(path, "Alpha", "Bravo", 1)
	assertList(t, f.list(opts), `droid d1 "Alpha" turns=1`+"\n")

	opts.NoCache = true
	assertList(t, f.list(opts), `droid d1 "Bravo" turns=1`+"\n")
}

func TestAgentFilterKeepsOtherAgentsCached(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	path := f.droidSession(proj, "d1", 1, obj{"title": "Alpha"}, droidUser("q"))
	f.codexSession(proj, "x1", 2, nil, "hello")
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	f.list(opts)
	f.rewriteSameSize(path, "Alpha", "Bravo", 1)

	codexOnly := opts
	codexOnly.Agents = []string{"codex"}
	assertList(t, f.list(codexOnly), `codex x1 "hello" turns=1`+"\n")

	assertList(t, f.list(opts), `
		codex x1 "hello" turns=1
		droid d1 "Alpha" turns=1
	`)
}

func TestCacheReparsesModifiedFiles(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	path := f.droidSession(proj, "d1", 1, obj{"title": "Alpha"}, droidUser("q"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	f.list(opts)

	f.rewriteSameSize(path, "Alpha", "Bravo", 2)
	assertList(t, f.list(opts), `droid d1 "Bravo" turns=1`+"\n")
}

func TestRebuildCacheIgnoresExistingEntries(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	path := f.droidSession(proj, "d1", 1, obj{"title": "Alpha"}, droidUser("q"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	f.list(opts)
	f.rewriteSameSize(path, "Alpha", "Bravo", 1)

	opts.RebuildCache = true
	assertList(t, f.list(opts), `droid d1 "Bravo" turns=1`+"\n")
	opts.RebuildCache = false
	assertList(t, f.list(opts), `droid d1 "Bravo" turns=1`+"\n")
}

func TestCacheFromOtherSchemaVersionIsDiscarded(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	path := f.droidSession(proj, "d1", 1, obj{"title": "Alpha"}, droidUser("q"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	f.list(opts)
	f.rewriteSameSize(path, "Alpha", "Bravo", 1)

	var doc map[string]any
	data, err := os.ReadFile(opts.CachePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["schemaVersion"] = -1
	data, _ = json.Marshal(doc)
	if err := os.WriteFile(opts.CachePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	assertList(t, f.list(opts), `droid d1 "Bravo" turns=1`+"\n")
}

func TestRepoSlugCachedAcrossRuns(t *testing.T) {
	f := newFixture(t)
	proj := f.project("cached")
	f.initRepo(proj, "https://github.com/team/cached.git")
	f.droidSession(proj, "d1", 1, nil, droidUser("hi"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")

	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=team/cached`+"\n")

	os.RemoveAll(filepath.Join(proj, ".git"))
	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=team/cached`+"\n")
}

func TestRefreshRecomputesSlugs(t *testing.T) {
	f := newFixture(t)
	proj := f.project("refresh")
	f.initRepo(proj, "https://github.com/old/name.git")
	f.droidSession(proj, "d1", 1, nil, droidUser("hi"))
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=old/name`+"\n")

	config := "[remote \"origin\"]\n\turl = https://github.com/new/name.git\n"
	os.WriteFile(filepath.Join(proj, ".git", "config"), []byte(config), 0o644)

	// Without refresh: cached slug persists
	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=old/name`+"\n")

	// With refresh: slug is recomputed
	opts.Refresh = true
	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=new/name`+"\n")

	// Subsequent run without refresh uses updated cache
	opts.Refresh = false
	assertList(t, f.list(opts), `droid d1 "hi" turns=1 repo=new/name`+"\n")
}

func TestEmptySlugCachedWithoutRewalk(t *testing.T) {
	f := newFixture(t)

	// Session in a repo with no remote: RepoDirName cached
	noRemote := f.project("local-repo")
	f.initRepoNoRemote(noRemote)
	f.droidSession(noRemote, "d-nr", 1, nil, droidUser("local"))

	// Session outside any repo: empty result cached
	noRepo := f.project("plain")
	f.droidSession(noRepo, "d-plain", 2, nil, droidUser("plain"))

	opts := f.options(noRemote)
	opts.Scope = index.All
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	assertList(t, f.list(opts), `
		droid d-plain "plain" turns=1
		droid d-nr "local" turns=1 repodir=local-repo
	`)

	// Remove .git from both to prove warm run uses cache (no re-walk)
	os.RemoveAll(filepath.Join(noRemote, ".git"))
	assertList(t, f.list(opts), `
		droid d-plain "plain" turns=1
		droid d-nr "local" turns=1 repodir=local-repo
	`)

	// Verify cache has RepoChecked set
	data, err := os.ReadFile(opts.CachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"repoChecked":true`) {
		t.Error("cache should contain repoChecked:true")
	}
}

func TestCodexTitlesAreFreshEvenWhenRolloutIsCached(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	f.codexSession(proj, "x1", 1, nil, "hello")
	opts := f.options(proj)
	opts.CachePath = filepath.Join(f.home, "cache", "index.json")
	assertList(t, f.list(opts), `codex x1 "hello" turns=1`+"\n")

	f.codexIndex(obj{"id": "x1", "thread_name": "Greeting"})
	assertList(t, f.list(opts), `codex x1 "Greeting" turns=1`+"\n")
}
