// Package agent defines the common session record and the provider contract
// that each supported coding agent implements.
package agent

import (
	"os"
	"path/filepath"
	"time"
)

// Session is one agent conversation discovered on disk.
//
// Fields above the "derived" marker are produced by a Provider's Parse and are
// safe to cache. Fields below it are computed by the index on every listing.
type Session struct {
	Agent        string `json:"agent"`
	ID           string `json:"id"`
	Path         string `json:"path"`
	Cwd          string `json:"cwd"`
	Title        string `json:"title,omitempty"`
	FirstMessage string `json:"firstMessage,omitempty"`
	Turns        int    `json:"turns"`
	ParentID     string `json:"parentId,omitempty"`
	Subagent     bool   `json:"subagent,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Headless     bool   `json:"headless,omitempty"`

	// derived
	Updated         time.Time  `json:"updated"`
	Resume          *Command   `json:"resume,omitempty"`
	Problem         string     `json:"problem,omitempty"`
	Orphan          bool       `json:"orphan,omitempty"`
	HiddenSubagents int        `json:"hiddenSubagents,omitempty"`
	Children        []*Session `json:"children,omitempty"`
}

// DisplayTitle is the agent's own title, falling back to the first user message.
func (s *Session) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return s.FirstMessage
}

// Command is a fully resolved resume invocation.
type Command struct {
	Bin  string   `json:"bin"`
	Args []string `json:"args"`
	Dir  string   `json:"dir"`
}

// Env abstracts the user's home directory and environment so tests can point
// providers at fixture trees.
type Env struct {
	Home   string
	Getenv func(string) string
}

// OSEnv returns the real process environment.
func OSEnv() Env {
	home, _ := os.UserHomeDir()
	return Env{Home: home, Getenv: os.Getenv}
}

// Dir returns the override from the named variable, or home joined with rel.
func (e Env) Dir(envVar string, rel ...string) string {
	if envVar != "" && e.Getenv != nil {
		if v := e.Getenv(envVar); v != "" {
			return v
		}
	}
	return filepath.Join(append([]string{e.Home}, rel...)...)
}

// Provider knows everything specific to one agent.
type Provider interface {
	Name() string
	Binary() string
	// Files lists every session file the agent has stored.
	Files(env Env) ([]string, error)
	// Parse reads one session file. A nil session with nil error means the file
	// is not a session and should be ignored.
	Parse(path string) (*Session, error)
	// ResumeArgs returns the arguments (excluding the binary) that resume s.
	ResumeArgs(s *Session) []string
}

// Enricher is implemented by providers that keep session data outside session
// files. Enrich runs after the cache, so its results are never cached.
type Enricher interface {
	Enrich(env Env, sessions []*Session)
}
