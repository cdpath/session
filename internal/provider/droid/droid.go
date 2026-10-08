// Package droid reads Factory Droid sessions from ~/.factory/sessions.
package droid

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/provider/shared"
)

type Provider struct{}

func (Provider) Name() string   { return "droid" }
func (Provider) Binary() string { return "droid" }

func root(env agent.Env) string {
	if env.Getenv != nil {
		if v := env.Getenv("FACTORY_HOME_OVERRIDE"); v != "" {
			return filepath.Join(v, ".factory", "sessions")
		}
	}
	return filepath.Join(env.Home, ".factory", "sessions")
}

func (Provider) Files(env agent.Env) ([]string, error) {
	return shared.ProjectSessions(root(env))
}

type line struct {
	Type             string  `json:"type"`
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Cwd              string  `json:"cwd"`
	CallingSessionID string  `json:"callingSessionId"`
	Message          message `json:"message"`
}

type message struct {
	Role          string `json:"role"`
	Visibility    string `json:"visibility"`
	HookEventName string `json:"hookEventName"`
	Content       []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (Provider) Parse(path string) (*agent.Session, error) {
	var s *agent.Session
	err := shared.EachLine(path, func(raw []byte) error {
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			if s == nil {
				return err
			}
			return nil // e.g. a partial line in a session still being written
		}
		if s == nil {
			if l.Type != "session_start" {
				return errors.New("first line is not session_start")
			}
			s = &agent.Session{Agent: "droid", ID: l.ID, Path: path, Cwd: l.Cwd}
			if l.Title != "New Session" {
				s.Title = shared.OneLine(l.Title)
			}
			if l.CallingSessionID != "" {
				s.Subagent = true
				s.ParentID = l.CallingSessionID
			}
			return nil
		}
		if l.Type != "message" || l.Message.Role != "user" {
			return nil
		}
		// llm_only/user_only messages and hook results are written by the harness.
		if l.Message.Visibility != "" || l.Message.HookEventName != "" {
			return nil
		}
		for _, c := range l.Message.Content {
			if c.Type == "text" && !shared.IsInjected(c.Text) && c.Text != "" {
				s.Turns++
				if s.FirstMessage == "" {
					s.FirstMessage = shared.OneLine(c.Text)
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (Provider) ResumeArgs(s *agent.Session) []string {
	return []string{"--resume", s.ID}
}
