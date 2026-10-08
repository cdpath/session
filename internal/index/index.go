// Package index discovers sessions across all providers and turns them into a
// filtered, sorted forest ready for display and resume.
package index

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/provider/claude"
	"github.com/cdpath/session/internal/provider/codex"
	"github.com/cdpath/session/internal/provider/droid"
	"github.com/cdpath/session/internal/provider/pi"
)

// Providers returns every supported agent in display order.
func Providers() []agent.Provider {
	return []agent.Provider{pi.Provider{}, claude.Provider{}, codex.Provider{}, droid.Provider{}}
}

type Options struct {
	Env       agent.Env
	Providers []agent.Provider // defaults to Providers()
	Cwd       string
	Global    bool
	Recursive bool
	Subagents bool     // show sub-agent sessions as a tree instead of hiding them
	Headless  bool     // include sessions started by SDKs and integrations
	Agents    []string // restrict to these agent names; empty means all
	LookPath  func(string) (string, error)

	CachePath    string // empty disables the cache
	NoCache      bool   // neither read nor write the cache
	RebuildCache bool   // ignore existing entries, then write a fresh cache
}

type Skip struct {
	Path string
	Err  error
}

type Result struct {
	Sessions []*agent.Session
	Skipped  []Skip
}

type parsed struct {
	provider agent.Provider
	session  *agent.Session
}

func List(opts Options) (Result, error) {
	providers := opts.Providers
	if providers == nil {
		providers = Providers()
	}
	if len(opts.Agents) > 0 {
		var keep []agent.Provider
		for _, p := range providers {
			if slices.Contains(opts.Agents, p.Name()) {
				keep = append(keep, p)
			}
		}
		providers = keep
	}
	target := normalize(opts.Cwd)
	scope := agent.Scope{Global: opts.Global, Recursive: opts.Recursive, Dirs: unique(filepath.Clean(opts.Cwd), target)}

	cache := cacheState{entries: map[string]cacheEntry{}}
	if opts.CachePath != "" && !opts.NoCache {
		cache.path = opts.CachePath
		if !opts.RebuildCache {
			cache.entries = loadCache(opts.CachePath)
		}
	}

	all, skipped, err := scan(opts.Env, providers, scope, cache)
	if err != nil {
		return Result{}, err
	}

	byKey := map[string]*agent.Session{}
	providerOf := map[*agent.Session]agent.Provider{}
	for _, p := range all {
		s := p.session
		if s.Turns == 0 || (s.Headless && !opts.Headless) {
			continue
		}
		byKey[key(s.Agent, s.ID)] = s
		providerOf[s] = p.provider
	}

	var roots []*agent.Session
	for _, p := range all {
		s := p.session
		if byKey[key(s.Agent, s.ID)] != s {
			continue
		}
		if !s.Subagent {
			roots = append(roots, s)
			continue
		}
		if parent := byKey[key(s.Agent, s.ParentID)]; parent != nil && parent != s {
			parent.Children = append(parent.Children, s)
		} else if opts.Subagents {
			s.Orphan = true
			roots = append(roots, s)
		}
	}

	visible := roots[:0]
	for _, s := range roots {
		// A sub-agent tree belongs to wherever its root session ran.
		if !opts.Global && !inScope(normalize(s.Cwd), target, opts.Recursive) {
			continue
		}
		visible = append(visible, s)
	}

	check := &problems{lookPath: opts.LookPath, bins: map[string]bool{}, dirs: map[string]bool{}}
	walk(visible, func(s *agent.Session) {
		p := providerOf[s]
		s.Resume = &agent.Command{Bin: p.Binary(), Args: p.ResumeArgs(s), Dir: s.Cwd}
		s.Problem = check.of(s)
	})
	if !opts.Subagents {
		for _, s := range visible {
			s.HiddenSubagents = countDescendants(s)
			s.Children = nil
		}
	}
	sortTree(visible)
	return Result{Sessions: visible, Skipped: skipped}, nil
}

func key(agentName, id string) string { return agentName + "\x00" + id }

func walk(ss []*agent.Session, fn func(*agent.Session)) {
	for _, s := range ss {
		fn(s)
		walk(s.Children, fn)
	}
}

func countDescendants(s *agent.Session) int {
	n := 0
	walk(s.Children, func(*agent.Session) { n++ })
	return n
}

func sortTree(ss []*agent.Session) {
	sortByUpdated(ss)
	for _, s := range ss {
		sortTree(s.Children)
	}
}

type cacheState struct {
	path    string // empty disables writing
	entries map[string]cacheEntry
}

func scan(env agent.Env, providers []agent.Provider, scope agent.Scope, cache cacheState) ([]parsed, []Skip, error) {
	type job struct {
		provider agent.Provider
		path     string
	}
	var jobs []job
	for _, p := range providers {
		files, err := p.Candidates(env, scope)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range files {
			jobs = append(jobs, job{p, f})
		}
	}

	var (
		mu      sync.Mutex
		out     []parsed
		skipped []Skip
		seen    = map[string]cacheEntry{}
		wg      sync.WaitGroup
		ch      = make(chan job)
	)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				info, err := os.Stat(j.path)
				var s *agent.Session
				var entry cacheEntry
				if err == nil {
					if old, ok := cache.entries[j.path]; ok && old.fresh(info) {
						entry, s = old, old.cached()
					} else if s, err = j.provider.Parse(j.path); err == nil {
						entry = newEntry(info, s)
					}
				}
				mu.Lock()
				switch {
				case err != nil:
					skipped = append(skipped, Skip{Path: j.path, Err: err})
				default:
					seen[j.path] = entry
					if s != nil {
						s.Updated = info.ModTime()
						out = append(out, parsed{j.provider, s})
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	if cache.path != "" {
		entries := seen
		if !scope.Global {
			// keep entries from other directories that this run did not visit
			entries = maps.Clone(cache.entries)
			maps.Copy(entries, seen)
		}
		if err := saveCache(cache.path, entries); err != nil {
			skipped = append(skipped, Skip{Path: cache.path, Err: err})
		}
	}

	for _, p := range providers {
		e, ok := p.(agent.Enricher)
		if !ok {
			continue
		}
		var mine []*agent.Session
		for _, x := range out {
			if x.provider.Name() == p.Name() {
				mine = append(mine, x.session)
			}
		}
		e.Enrich(env, mine)
	}
	return out, skipped, nil
}

// problems explains why a session can't be resumed, memoizing PATH and
// directory checks across sessions.
type problems struct {
	lookPath func(string) (string, error)
	bins     map[string]bool
	dirs     map[string]bool
}

func (p *problems) of(s *agent.Session) string {
	bin, dir := s.Resume.Bin, s.Resume.Dir
	ok, known := p.bins[bin]
	if !known {
		ok = true
		if p.lookPath != nil {
			_, err := p.lookPath(bin)
			ok = err == nil
		}
		p.bins[bin] = ok
	}
	if !ok {
		return bin + " is not installed"
	}
	exists, known := p.dirs[dir]
	if !known {
		info, err := os.Stat(dir)
		exists = err == nil && info.IsDir()
		p.dirs[dir] = exists
	}
	if !exists {
		return "directory no longer exists: " + dir
	}
	return ""
}

func sortByUpdated(ss []*agent.Session) {
	sort.SliceStable(ss, func(i, j int) bool {
		if !ss[i].Updated.Equal(ss[j].Updated) {
			return ss[i].Updated.After(ss[j].Updated)
		}
		return ss[i].ID < ss[j].ID
	})
}

// normalize cleans p and resolves symlinks when the path still exists, so
// /tmp and /private/tmp (or a symlinked project) compare equal.
func normalize(p string) string {
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func inScope(cwd, target string, recursive bool) bool {
	if cwd == target {
		return true
	}
	if !recursive {
		return false
	}
	return strings.HasPrefix(cwd, strings.TrimSuffix(target, string(filepath.Separator))+string(filepath.Separator))
}

func unique(ss ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
