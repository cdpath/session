// Package codex reads OpenAI Codex CLI rollouts from ~/.codex/sessions.
package codex

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/provider/shared"
)

type Provider struct{}

func (Provider) Name() string   { return "codex" }
func (Provider) Binary() string { return "codex" }

func home(env agent.Env) string { return env.Dir("CODEX_HOME", ".codex") }

// Candidates always returns every rollout: Codex organises files by date, so
// the cwd is only known after reading each file's first line.
func (Provider) Candidates(env agent.Env, _ agent.Scope) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(home(env), "sessions"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

type line struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type sessionMeta struct {
	ID             string          `json:"id"`
	Cwd            string          `json:"cwd"`
	Originator     string          `json:"originator"`
	Source         json.RawMessage `json:"source"`
	ThreadSource   string          `json:"thread_source"`
	ParentThreadID string          `json:"parent_thread_id"`
}

type event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (Provider) Parse(path string) (*agent.Session, error) {
	var s *agent.Session
	err := shared.EachLine(path, func(raw []byte) error {
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			if s == nil {
				return err
			}
			return nil
		}
		if s == nil {
			if l.Type != "session_meta" {
				return errors.New("first line is not session_meta")
			}
			var m sessionMeta
			if err := json.Unmarshal(l.Payload, &m); err != nil {
				return err
			}
			s = &agent.Session{
				Agent:    "codex",
				ID:       m.ID,
				Path:     path,
				Cwd:      m.Cwd,
				Headless: !interactive(m.Originator),
			}
			if m.ThreadSource == "subagent" {
				s.Subagent = true
				s.ParentID = m.ParentThreadID
				s.Tag = subagentKind(m.Source)
			}
			return nil
		}
		if l.Type != "event_msg" {
			return nil
		}
		var e event
		if json.Unmarshal(l.Payload, &e) == nil && e.Type == "user_message" && strings.TrimSpace(e.Message) != "" {
			s.Turns++
			if s.FirstMessage == "" {
				s.FirstMessage = shared.OneLine(e.Message)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// interactive reports whether a session was started by a human-facing client.
// SDK and tool integrations (agent SDKs, repoprompt, other agents calling
// Codex) count as headless.
func interactive(originator string) bool {
	return originator == "" || originator == "codex-tui" || strings.Contains(strings.ToLower(originator), "desktop")
}

// subagentKind reads source values such as {"subagent":"review"} or
// {"subagent":{"other":"guardian"}}.
func subagentKind(raw json.RawMessage) string {
	var src struct {
		Subagent json.RawMessage `json:"subagent"`
	}
	if json.Unmarshal(raw, &src) != nil || src.Subagent == nil {
		return ""
	}
	var kind string
	if json.Unmarshal(src.Subagent, &kind) == nil {
		return kind
	}
	var other map[string]string
	if json.Unmarshal(src.Subagent, &other) == nil {
		for _, v := range other {
			return v
		}
	}
	return ""
}

// Enrich applies thread names from session_index.jsonl, where Codex stores
// titles separately from rollouts. The last entry for an id wins.
func (Provider) Enrich(env agent.Env, sessions []*agent.Session) {
	if len(sessions) == 0 {
		return
	}
	names := map[string]string{}
	path := filepath.Join(home(env), "session_index.jsonl")
	if _, err := os.Stat(path); err != nil {
		return
	}
	_ = shared.EachLine(path, func(raw []byte) error {
		var e struct {
			ID         string `json:"id"`
			ThreadName string `json:"thread_name"`
		}
		if json.Unmarshal(raw, &e) == nil && e.ID != "" && e.ThreadName != "" {
			names[e.ID] = e.ThreadName
		}
		return nil
	})
	for _, s := range sessions {
		if n, ok := names[s.ID]; ok {
			s.Title = shared.OneLine(n)
		}
	}
}

func (Provider) ResumeArgs(s *agent.Session) []string {
	return []string{"resume", s.ID}
}
