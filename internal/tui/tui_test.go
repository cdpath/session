package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cdpath/session/internal/agent"
)

func sessions(agentName string, n int) []*agent.Session {
	out := make([]*agent.Session, n)
	for i := range out {
		out[i] = &agent.Session{Agent: agentName, ID: fmt.Sprintf("%s-%d", agentName, i), Title: "task", Turns: 1, Cwd: "/p", Updated: time.Now()}
	}
	return out
}

// Switching from an agent whose list fits on one page to one that needs
// several used to make the view one line taller than the window, which pushed
// the header off screen.
func TestHeaderStaysVisibleWhenSwitchingAgents(t *testing.T) {
	const width, height = 140, 45
	cfg := Config{Heading: "all directories", Global: true, Agents: []string{"small", "big"}}
	cfg.Sessions = append(sessions("small", 5), sessions("big", 200)...)

	var m tea.Model = newModel(cfg)
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	for _, want := range []string{"all agents", "small", "big", "all agents"} {
		lines := strings.Split(m.View(), "\n")
		if len(lines) > height {
			t.Errorf("%s: view has %d lines, window has %d", want, len(lines), height)
		}
		if !strings.Contains(lines[0], "all directories · "+want) {
			t.Errorf("%s: first line = %q, want the header", want, strings.TrimSpace(lines[0]))
		}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
}

// Same overflow via bubbles' own filter: narrow to one page, then clear it.
func TestHeaderStaysVisibleAfterClearingFilter(t *testing.T) {
	const width, height = 140, 45
	cfg := Config{Heading: "here", Agents: []string{"a"}}
	cfg.Sessions = append(sessions("a", 200), &agent.Session{Agent: "a", ID: "unique", Title: "zebra", Turns: 1, Cwd: "/p", Updated: time.Now()})

	var m tea.Model = newModel(cfg)
	m = send(m, tea.WindowSizeMsg{Width: width, Height: height})
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zebra")})
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter}) // applies the filter
	m = send(m, tea.KeyMsg{Type: tea.KeyEsc})   // clears it

	lines := strings.Split(m.View(), "\n")
	if len(lines) > height {
		t.Errorf("view has %d lines, window has %d", len(lines), height)
	}
	if !strings.Contains(lines[0], "here · all agents") {
		t.Errorf("first line = %q, want the header", strings.TrimSpace(lines[0]))
	}
}

// send delivers msg and then the messages produced by its command, the way
// the bubbletea runtime would (filtering results arrive via a command).
func send(m tea.Model, msg tea.Msg) tea.Model {
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		var cmd tea.Cmd
		m, cmd = m.Update(queue[0])
		queue = append(queue[1:], run(cmd)...)
	}
	return m
}

func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range batch {
				out = append(out, run(c)...)
			}
			return out
		}
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	case <-time.After(50 * time.Millisecond):
		return nil // timers such as cursor blink
	}
}
