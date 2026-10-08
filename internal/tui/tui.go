// Package tui is the interactive session picker.
package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/display"
	"github.com/cdpath/session/internal/index"
)

var agentColors = map[string]lipgloss.Color{
	"pi":     "205",
	"claude": "208",
	"codex":  "42",
	"droid":  "39",
}

var (
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	selectedText = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Config controls what the picker shows.
type Config struct {
	Sessions []*agent.Session // every directory; the picker filters by scope
	Dir      string           // starting directory that scopes are relative to
	Scope    index.Scope      // scope shown first
	Home     string
	Agents   []string // agent names to cycle through with Tab
}

type item struct {
	row display.Row
}

func (i item) FilterValue() string {
	s := i.row.Session
	return s.Agent + " " + display.Title(s) + " " + s.Cwd
}

type delegate struct {
	scope  index.Scope
	scoper *index.Scoper
	home   string
	now    time.Time
}

func (d delegate) cwd(s *agent.Session) string {
	switch d.scope {
	case index.Subdirs:
		if rel, ok := d.scoper.Rel(s.Cwd); ok {
			return rel
		}
		return display.ShortPath(s.Cwd, d.home)
	case index.All:
		return display.ShortPath(s.Cwd, d.home)
	}
	return ""
}

func (delegate) Height() int                         { return 1 }
func (delegate) Spacing() int                        { return 0 }
func (delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it := li.(item)
	s := it.row.Session
	selected := index == m.Index()

	cursor := "  "
	if selected {
		cursor = cursorStyle.Render("> ")
	}
	tag := lipgloss.NewStyle().Foreground(agentColors[s.Agent]).Width(7).Render(s.Agent)
	meta := fmt.Sprintf("%9s %4d  ", display.Ago(s.Updated, d.now), s.Turns)

	width := m.Width() - 2
	title := it.row.Prefix + display.Title(s)
	var cwd string
	if c := d.cwd(s); c != "" {
		cwd = "  " + c
	}
	room := width - lipgloss.Width(tag) - lipgloss.Width(meta) - lipgloss.Width(cwd)
	if room < 10 {
		room, cwd = width-lipgloss.Width(tag)-lipgloss.Width(meta), ""
	}
	title = ansi.Truncate(title, max(room, 1), "…")
	pad := strings.Repeat(" ", max(room-lipgloss.Width(title), 0))

	switch {
	case s.Problem != "":
		fmt.Fprint(w, cursor+dimStyle.Render(s.Agent+strings.Repeat(" ", max(7-len(s.Agent), 0))+meta+title+pad+cwd))
	case selected:
		fmt.Fprint(w, cursor+tag+meta+selectedText.Render(title)+pad+dimStyle.Render(cwd))
	default:
		fmt.Fprint(w, cursor+tag+meta+title+pad+dimStyle.Render(cwd))
	}
}

type model struct {
	cfg      Config
	list     list.Model
	scoper   *index.Scoper
	scope    index.Scope
	agentIdx int // 0 = all agents
	choice   *agent.Session
	tab      key.Binding
	scopeKey key.Binding
	enter    key.Binding
	now      time.Time
}

// Run shows the picker and returns the chosen session, or nil if the user quit.
func Run(cfg Config) (*agent.Session, error) {
	final, err := tea.NewProgram(newModel(cfg), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	return final.(model).choice, nil
}

func newModel(cfg Config) model {
	m := model{
		cfg:      cfg,
		scoper:   index.NewScoper(cfg.Dir),
		scope:    cfg.Scope,
		tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "agent")),
		scopeKey: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "scope")),
		enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "resume")),
		now:      time.Now(),
	}
	l := list.New(nil, m.delegate(), 0, 0)
	l.SetStatusBarItemName("session", "sessions")
	l.SetShowHelp(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	l.AdditionalShortHelpKeys = func() []key.Binding { return []key.Binding{m.enter, m.tab, m.scopeKey} }
	l.AdditionalFullHelpKeys = l.AdditionalShortHelpKeys
	m.list = l
	m.applyFilters()
	return m
}

func (m *model) delegate() delegate {
	return delegate{scope: m.scope, scoper: m.scoper, home: m.cfg.Home, now: m.now}
}

func (m *model) currentAgent() string {
	if m.agentIdx == 0 {
		return ""
	}
	return m.cfg.Agents[m.agentIdx-1]
}

// applyFilters rebuilds the items for the current scope and agent, keeping the
// text filter and, when it is still listed, the selected session.
func (m *model) applyFilters() {
	var selected *agent.Session
	if it, ok := m.list.SelectedItem().(item); ok {
		selected = it.row.Session
	}
	want := m.currentAgent()
	var roots []*agent.Session
	for _, s := range m.scoper.Filter(m.cfg.Sessions, m.scope) {
		if want == "" || s.Agent == want {
			roots = append(roots, s)
		}
	}
	rows := display.Flatten(roots)
	items := make([]list.Item, len(rows))
	for i, r := range rows {
		items[i] = item{row: r}
	}
	m.list.SetDelegate(m.delegate())
	// With a text filter applied, the list re-filters through a command. Run it
	// now so the selection can be restored against the filtered items.
	if cmd := m.list.SetItems(items); cmd != nil {
		m.list, _ = m.list.Update(cmd())
	}
	m.list.Title = m.title()
	m.fixPagination() // Select uses the page size
	m.list.Select(0)
	for i, li := range m.list.VisibleItems() {
		if li.(item).row.Session == selected {
			m.list.Select(i)
			break
		}
	}
}

// fixPagination works around bubbles' list sizing a page with the pagination
// height of the previous item set: going from one page to several (switching
// agents, clearing a filter) made the view a line taller than the window, and
// bubbletea then dropped the top line, i.e. the header. Re-applying the size
// recomputes the page with the current page count.
func (m *model) fixPagination() {
	m.list.SetSize(m.list.Width(), m.list.Height())
}

func (m *model) title() string {
	label := "all agents"
	if a := m.currentAgent(); a != "" {
		label = a
	}
	return fmt.Sprintf("%s · %s", m.scopeLabel(), label)
}

func (m *model) scopeLabel() string {
	switch m.scope {
	case index.Subdirs:
		return display.ShortPath(m.cfg.Dir, m.cfg.Home) + " w/ subdirs"
	case index.All:
		return "all directories"
	}
	return display.ShortPath(m.cfg.Dir, m.cfg.Home)
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if m.list.SettingFilter() {
			break
		}
		switch {
		case key.Matches(msg, m.tab):
			m.agentIdx = (m.agentIdx + 1) % (len(m.cfg.Agents) + 1)
			m.list.ResetFilter()
			m.applyFilters()
			return m, nil
		case key.Matches(msg, m.scopeKey):
			m.scope = m.scope.Next()
			m.applyFilters()
			return m, nil
		case key.Matches(msg, m.enter):
			it, ok := m.list.SelectedItem().(item)
			if !ok {
				return m, nil
			}
			if p := it.row.Session.Problem; p != "" {
				return m, m.list.NewStatusMessage(errorStyle.Render("cannot resume: " + p))
			}
			m.choice = it.row.Session
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.fixPagination()
	return m, cmd
}

func (m model) View() string {
	v := m.list.View()
	if len(m.list.VisibleItems()) > 0 || m.list.SettingFilter() {
		return v
	}
	// The list has no hook for its empty text, so swap the rendered line.
	stock := m.list.Styles.NoItems.Render("No sessions.")
	hint := ansi.Truncate(m.emptyHint(), max(m.list.Width()-4, 1), "…")
	return strings.Replace(v, stock, "  "+m.list.Styles.NoItems.Render(hint), 1)
}

func (m *model) emptyHint() string {
	a := m.currentAgent()
	switch {
	case a == "" && m.scope == index.All:
		return "No sessions anywhere"
	case a == "":
		return "No sessions here · press s to widen scope"
	case m.scope == index.All:
		return fmt.Sprintf("No %s sessions in %s · tab next agent", a, m.scopeLabel())
	}
	return fmt.Sprintf("No %s sessions in %s · s widen scope · tab next agent", a, m.scopeLabel())
}
