// Package pi reads pi-coding-agent sessions from ~/.pi/agent/sessions.
package pi

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/provider/shared"
)

type Provider struct{}

func (Provider) Name() string   { return "pi" }
func (Provider) Binary() string { return "pi" }

func root(env agent.Env) string {
	if env.Getenv != nil {
		if v := env.Getenv("PI_CODING_AGENT_SESSION_DIR"); v != "" {
			return v
		}
	}
	return filepath.Join(env.Dir("PI_CODING_AGENT_DIR", ".pi", "agent"), "sessions")
}

func (Provider) Candidates(env agent.Env, scope agent.Scope) ([]string, error) {
	return shared.ProjectSessions(root(env), scope)
}

type line struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	Cwd           string `json:"cwd"`
	ParentSession string `json:"parentSession"`
	Name          string `json:"name"`
	Message       struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

func (Provider) Parse(path string) (*agent.Session, error) {
	var s *agent.Session
	var name string
	err := shared.EachLine(path, func(raw []byte) error {
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			if s == nil {
				return err
			}
			return nil
		}
		if s == nil {
			if l.Type != "session" {
				return errors.New("first line is not a session header")
			}
			s = &agent.Session{Agent: "pi", ID: l.ID, Path: path, Cwd: l.Cwd}
			// Sessions spawned by pi's sub-agent extension point at the parent
			// session file.
			if l.ParentSession != "" {
				s.Subagent = true
				s.ParentID = idFromPath(l.ParentSession)
			}
			return nil
		}
		switch l.Type {
		case "session_info":
			if l.Name != "" {
				name = l.Name
			}
		case "message":
			if l.Message.Role != "user" {
				return nil
			}
			if text := userText(l.Message.Content); text != "" {
				s.Turns++
				if s.FirstMessage == "" {
					s.FirstMessage = shared.OneLine(text)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, nil
	}
	if s.Subagent {
		// sub-agent names look like "general-purpose#1a2b3c4d"
		s.Tag, _, _ = strings.Cut(name, "#")
	} else {
		s.Title = shared.OneLine(name)
	}
	return s, nil
}

// idFromPath extracts the session id from "<timestamp>_<id>.jsonl".
func idFromPath(p string) string {
	stem := shared.FileStem(p)
	if i := strings.LastIndex(stem, "_"); i >= 0 {
		return stem[i+1:]
	}
	return stem
}

func userText(raw json.RawMessage) string {
	var str string
	if json.Unmarshal(raw, &str) == nil {
		if shared.IsInjected(str) {
			return ""
		}
		return strings.TrimSpace(str)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	for _, p := range parts {
		if p.Type == "text" && !shared.IsInjected(p.Text) && strings.TrimSpace(p.Text) != "" {
			return strings.TrimSpace(p.Text)
		}
	}
	return ""
}

// ResumeArgs uses the full file path: pi also accepts partial ids, which can
// be ambiguous.
func (Provider) ResumeArgs(s *agent.Session) []string {
	return []string{"--session", s.Path}
}
