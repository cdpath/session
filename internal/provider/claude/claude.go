// Package claude reads Claude Code sessions from ~/.claude/projects.
package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/provider/shared"
)

type Provider struct{}

func (Provider) Name() string   { return "claude" }
func (Provider) Binary() string { return "claude" }

func root(env agent.Env) string {
	return filepath.Join(env.Dir("CLAUDE_CONFIG_DIR", ".claude"), "projects")
}

// Candidates returns main sessions (<project>/<uuid>.jsonl) and sub-agent
// transcripts (<project>/<parent-uuid>/subagents/agent-*.jsonl).
func (Provider) Candidates(env agent.Env, scope agent.Scope) ([]string, error) {
	dirs, err := shared.ProjectDirs(root(env), scope)
	if err != nil {
		return nil, err
	}
	files := shared.GlobJSONL(dirs)
	for _, d := range dirs {
		subs, _ := filepath.Glob(filepath.Join(d, "*", "subagents", "agent-*.jsonl"))
		files = append(files, subs...)
	}
	return files, nil
}

type line struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId"`
	Cwd         string          `json:"cwd"`
	Entrypoint  string          `json:"entrypoint"`
	IsMeta      bool            `json:"isMeta"`
	IsSidechain bool            `json:"isSidechain"`
	AITitle     string          `json:"aiTitle"`
	CustomTitle string          `json:"customTitle"`
	Message     json.RawMessage `json:"message"`
}

type meta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
}

func (Provider) Parse(path string) (*agent.Session, error) {
	subagent := filepath.Base(filepath.Dir(path)) == "subagents"
	s := &agent.Session{Agent: "claude", Path: path, ID: shared.FileStem(path)}
	if subagent {
		s.Subagent = true
		s.ID = strings.TrimPrefix(s.ID, "agent-")
		s.ParentID = filepath.Base(filepath.Dir(filepath.Dir(path)))
		if data, err := os.ReadFile(strings.TrimSuffix(path, ".jsonl") + ".meta.json"); err == nil {
			var m meta
			if json.Unmarshal(data, &m) == nil {
				s.Title = shared.OneLine(m.Description)
				s.Tag = m.AgentType
			}
		}
	}

	var aiTitle, customTitle string
	err := shared.EachLine(path, func(raw []byte) error {
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			return nil // tolerate partial or unknown lines
		}
		switch l.Type {
		case "ai-title":
			aiTitle = l.AITitle
		case "custom-title":
			customTitle = l.CustomTitle
		case "user", "assistant":
			if s.Cwd == "" {
				s.Cwd = l.Cwd
			}
			if !subagent && l.SessionID != "" {
				s.ID = l.SessionID
			}
			if l.Entrypoint != "" && strings.HasPrefix(l.Entrypoint, "sdk") {
				s.Headless = true
			}
			if l.Type != "user" || l.IsMeta || (l.IsSidechain && !subagent) {
				return nil
			}
			if text := userText(l.Message); text != "" {
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
	if s.Cwd == "" {
		return nil, nil
	}
	if !subagent {
		s.Title = shared.OneLine(firstNonEmpty(customTitle, aiTitle))
	}
	return s, nil
}

// userText extracts typed text from a user message, ignoring tool results and
// harness-injected content such as command echoes.
func userText(raw json.RawMessage) string {
	var m struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	var str string
	if json.Unmarshal(m.Content, &str) == nil {
		if shared.IsInjected(str) {
			return ""
		}
		return strings.TrimSpace(str)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(m.Content, &parts) != nil {
		return ""
	}
	for _, p := range parts {
		if p.Type == "text" && !shared.IsInjected(p.Text) && strings.TrimSpace(p.Text) != "" {
			return strings.TrimSpace(p.Text)
		}
	}
	return ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// ResumeArgs resumes the parent for sub-agents: Claude sub-agent transcripts
// are not standalone sessions.
func (Provider) ResumeArgs(s *agent.Session) []string {
	id := s.ID
	if s.Subagent {
		id = s.ParentID
	}
	return []string{"--resume", id}
}
