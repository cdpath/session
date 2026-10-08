package index

import (
	"path/filepath"
	"strings"

	"github.com/cdpath/session/internal/agent"
)

// Scope says which directories a listing covers, relative to a starting
// directory.
type Scope int

const (
	Here    Scope = iota // the starting directory only
	Subdirs              // the starting directory and everything below it
	All                  // every directory
	numScopes
)

// Next returns the scope after s in the cycle Here → Subdirs → All → Here.
func (s Scope) Next() Scope { return (s + 1) % numScopes }

// Scoper matches sessions against a starting directory, resolving each
// distinct path once.
type Scoper struct {
	dir      string
	resolved map[string]string
}

// NewScoper returns a Scoper for scopes relative to dir.
func NewScoper(dir string) *Scoper {
	sc := &Scoper{resolved: map[string]string{}}
	sc.dir = sc.resolve(dir)
	return sc
}

// Filter returns the root sessions whose cwd falls within scope. Sub-agent
// trees go wherever their root goes.
func (sc *Scoper) Filter(roots []*agent.Session, scope Scope) []*agent.Session {
	var out []*agent.Session
	for _, s := range roots {
		if sc.within(s.Cwd, scope) {
			out = append(out, s)
		}
	}
	return out
}

// Rel returns p relative to the starting directory ("." or "./sub/dir"), or
// false when p is not inside it.
func (sc *Scoper) Rel(p string) (string, bool) {
	r := sc.resolve(p)
	if r == sc.dir {
		return ".", true
	}
	if !sc.within(p, Subdirs) {
		return "", false
	}
	return "./" + strings.TrimPrefix(r, sc.prefix()), true
}

func (sc *Scoper) within(p string, scope Scope) bool {
	if scope == All {
		return true
	}
	r := sc.resolve(p)
	return r == sc.dir || scope == Subdirs && strings.HasPrefix(r, sc.prefix())
}

func (sc *Scoper) prefix() string {
	return strings.TrimSuffix(sc.dir, string(filepath.Separator)) + string(filepath.Separator)
}

func (sc *Scoper) resolve(p string) string {
	r, ok := sc.resolved[p]
	if !ok {
		r = normalize(p)
		sc.resolved[p] = r
	}
	return r
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
