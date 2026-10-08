package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/index"
)

func sessions(agentName string, n int) []*agent.Session {
	out := make([]*agent.Session, n)
	for i := range out {
		out[i] = &agent.Session{Agent: agentName, ID: fmt.Sprintf("%s-%d", agentName, i), Title: "task", Turns: 1, Cwd: "/p", Updated: time.Now()}
	}
	return out
}

const home = "/home/u"

func at(agentName, id, title, cwd string, hour int) *agent.Session {
	return &agent.Session{Agent: agentName, ID: id, Title: title, Turns: 1, Cwd: home + cwd,
		Updated: time.Now().Add(-time.Duration(hour) * time.Hour)}
}

// project has sessions in ~/repo, ~/repo/web and ~/other, newest first.
func project(scope index.Scope) Config {
	return Config{
		Sessions: []*agent.Session{
			at("droid", "d1", "alpha", "/repo", 1),
			at("codex", "x1", "bravo", "/repo/web", 2),
			at("droid", "d2", "charlie", "/other", 3),
		},
		Dir:    home + "/repo",
		Scope:  scope,
		Home:   home,
		Agents: []string{"codex", "droid"},
	}
}

func start(cfg Config) tea.Model {
	return send(newModel(cfg), tea.WindowSizeMsg{Width: 140, Height: 30})
}

func press(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func header(m tea.Model) string {
	return strings.TrimSpace(strings.Split(m.View(), "\n")[0])
}

// rows returns the session rows of the view, trimmed, in display order.
func rows(m tea.Model) []string {
	var out []string
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, " ago ") {
			out = append(out, strings.Join(strings.Fields(l), " "))
		}
	}
	return out
}

func assertRows(t *testing.T, m tea.Model, want ...string) {
	t.Helper()
	got := rows(m)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestScopeKeyCyclesScopes(t *testing.T) {
	m := start(project(index.Here))
	if h := header(m); h != "~/repo · all agents" {
		t.Errorf("header = %q", h)
	}
	if !strings.Contains(m.View(), "s scope") {
		t.Error("help does not mention s scope")
	}
	assertRows(t, m, "> droid 1h ago 1 alpha")

	m = send(m, press("s"))
	if h := header(m); h != "~/repo w/ subdirs · all agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m,
		"> droid 1h ago 1 alpha .",
		"codex 2h ago 1 bravo ./web")

	m = send(m, press("s"))
	if h := header(m); h != "all directories · all agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m,
		"> droid 1h ago 1 alpha ~/repo",
		"codex 2h ago 1 bravo ~/repo/web",
		"droid 3h ago 1 charlie ~/other")

	m = send(m, press("s"))
	if h := header(m); h != "~/repo · all agents" {
		t.Errorf("header = %q", h)
	}
}

// codex has no session in ~/repo itself, yet Tab still offers it.
func TestAgentFilterSurvivesScopeSwitch(t *testing.T) {
	m := start(project(index.Here))
	m = send(m, press("tab"))
	if h := header(m); h != "~/repo · codex" {
		t.Errorf("header = %q", h)
	}
	m = send(m, press("s"))
	if h := header(m); h != "~/repo w/ subdirs · codex" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m, "> codex 2h ago 1 bravo ./web")
}

func TestAppliedTextFilterSurvivesScopeSwitch(t *testing.T) {
	m := start(project(index.Subdirs))
	m = send(m, press("/"))
	m = send(m, press("bravo"))
	m = send(m, press("enter"))
	m = send(m, press("s"))
	if h := header(m); h != "all directories · all agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m, "> codex 2h ago 1 bravo ~/repo/web")

	// bravo is not in ~/repo itself, so nothing matches there
	m = send(m, press("s"))
	if !strings.Contains(m.View(), "No sessions here · press s to widen scope") {
		t.Errorf("no widening hint:\n%s", m.View())
	}
}

func TestSelectionSurvivesScopeSwitch(t *testing.T) {
	cfg := project(index.Subdirs)
	charlie := cfg.Sessions[2]
	charlie.Updated = time.Now().Add(-30 * time.Minute)
	cfg.Sessions = append([]*agent.Session{charlie}, cfg.Sessions[:2]...)
	m := start(cfg)
	m = send(m, press("j"))
	m = send(m, press("s"))
	assertRows(t, m,
		"droid 30m ago 1 charlie ~/other",
		"droid 1h ago 1 alpha ~/repo",
		"> codex 2h ago 1 bravo ~/repo/web")

	// bravo is not in ~/repo itself, so the cursor falls back to the first row
	m = send(m, press("s"))
	assertRows(t, m, "> droid 1h ago 1 alpha")
}

func TestScopeKeyTypesIntoFilter(t *testing.T) {
	m := start(project(index.Here))
	m = send(m, press("/"))
	m = send(m, press("s"))
	if !strings.Contains(m.View(), "Filter: s") {
		t.Errorf("filter input does not contain s:\n%s", m.View())
	}
	m = send(m, press("esc"))
	if h := header(m); h != "~/repo · all agents" {
		t.Errorf("header = %q, scope changed while typing a filter", h)
	}
}

func TestEmptyScopeSuggestsWidening(t *testing.T) {
	cfg := project(index.Here)
	cfg.Sessions = cfg.Sessions[2:] // only ~/other
	m := start(cfg)
	if !strings.Contains(m.View(), "No sessions here · press s to widen scope") {
		t.Errorf("no widening hint:\n%s", m.View())
	}
	m = send(m, press("s"))
	m = send(m, press("s"))
	assertRows(t, m, "> droid 3h ago 1 charlie ~/other")
}

func TestEmptyAgentInScopeSuggestsBothWaysOut(t *testing.T) {
	m := send(start(project(index.Here)), press("tab"))
	if !strings.Contains(m.View(), "No codex sessions in ~/repo · s widen scope · tab next agent") {
		t.Errorf("no hint:\n%s", m.View())
	}
	m = send(m, press("s"))
	assertRows(t, m, "> codex 2h ago 1 bravo ./web")
}

func TestEmptyEverywhereDoesNotSuggestWidening(t *testing.T) {
	m := start(Config{Dir: home + "/repo", Scope: index.All, Home: home})
	if v := m.View(); !strings.Contains(v, "No sessions anywhere") || strings.Contains(v, "widen") {
		t.Errorf("unexpected empty view:\n%s", v)
	}
}

func TestHeaderStaysVisibleWhenWideningScope(t *testing.T) {
	const height = 45
	cfg := Config{Dir: "/here", Agents: []string{"a"}}
	for i, s := range sessions("a", 205) {
		if i < 5 {
			s.Cwd = "/here"
		}
		cfg.Sessions = append(cfg.Sessions, s)
	}
	var m tea.Model = send(newModel(cfg), tea.WindowSizeMsg{Width: 140, Height: height})
	for _, want := range []string{"/here ·", "/here w/ subdirs ·", "all directories ·"} {
		lines := strings.Split(m.View(), "\n")
		if len(lines) > height {
			t.Errorf("%s: view has %d lines, window has %d", want, len(lines), height)
		}
		if !strings.Contains(lines[0], want) {
			t.Errorf("first line = %q, want %q", strings.TrimSpace(lines[0]), want)
		}
		m = send(m, press("s"))
	}
}

func TestStartingScopeComesFromConfig(t *testing.T) {
	m := start(project(index.All))
	if h := header(m); h != "all directories · all agents" {
		t.Errorf("header = %q", h)
	}
	if n := len(rows(m)); n != 3 {
		t.Errorf("%d rows, want 3", n)
	}
}

// Switching from an agent whose list fits on one page to one that needs
// several used to make the view one line taller than the window, which pushed
// the header off screen.
func TestHeaderStaysVisibleWhenSwitchingAgents(t *testing.T) {
	const width, height = 140, 45
	cfg := Config{Scope: index.All, Agents: []string{"small", "big"}}
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
	cfg := Config{Dir: "/p", Agents: []string{"a"}}
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
	if !strings.Contains(lines[0], "/p · all agents") {
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
