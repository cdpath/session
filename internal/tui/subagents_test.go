package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/index"
)

func with(parent *agent.Session, children ...*agent.Session) *agent.Session {
	parent.Children = children
	return parent
}

// family is, newest first, in ~/repo:
//
//	plan (droid) ── build ── probe
//	             └─ docs
//	solo (codex)
//	ship (claude) ── lint
//	stray (droid, orphaned sub-agent)
func family() Config {
	cfg := project(index.Here)
	stray := at("droid", "stray", "stray", "/repo", 8)
	stray.Subagent, stray.Orphan = true, true
	cfg.Sessions = []*agent.Session{
		with(at("droid", "plan", "plan", "/repo", 1),
			with(at("droid", "build", "build", "/repo", 2), at("droid", "probe", "probe", "/repo", 3)),
			at("droid", "docs", "docs", "/repo", 4)),
		at("codex", "solo", "solo", "/repo", 5),
		with(at("claude", "ship", "ship", "/repo", 6), at("claude", "lint", "lint", "/repo", 7)),
		stray,
	}
	cfg.Agents = []string{"codex", "droid", "claude"}
	return cfg
}

func TestSubagentsStartCollapsed(t *testing.T) {
	assertRows(t, start(family()),
		"> droid 1h ago 1 plan +3 sub",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")
}

func TestRightExpandsOneLevelAtATime(t *testing.T) {
	m := send(start(family()), press("right"))
	assertRows(t, m,
		"> droid 1h ago 1 plan −3 sub",
		"├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")

	m = send(m, press("j"))
	m = send(m, press("l"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub",
		"> ├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")
}

func TestLeftCollapses(t *testing.T) {
	m := start(family())
	m = send(m, press("right"))
	m = send(m, press("j"))
	m = send(m, press("right")) // plan and build are open, cursor on build

	m = send(m, press("left"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub",
		"> ├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")

	// on a sub-agent row, ← goes to the parent and closes it
	m = send(m, press("h"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")

	// on a closed top-level row, ← does nothing
	m = send(m, press("left"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")
}

func TestCollapsingFromGrandchildClosesItsParentOnly(t *testing.T) {
	m := start(family())
	for _, k := range []string{"right", "j", "right", "j"} { // cursor on probe
		m = send(m, press(k))
	}
	m = send(m, press("left"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub",
		"> ├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")
}

func TestFShowsOnlySessionsWithSubagentsExpanded(t *testing.T) {
	m := send(start(family()), press("f"))
	if h := header(m); h != "~/repo · all agents · with sub-agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m,
		"> droid 1h ago 1 plan −3 sub",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"claude 6h ago 1 ship −1 sub",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan)")

	// ← and → still work inside f
	m = send(m, press("left"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub",
		"claude 6h ago 1 ship −1 sub",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan)")
}

func TestFOffRestoresEarlierExpansion(t *testing.T) {
	m := start(family())
	m = send(m, press("G")) // last row: ship
	m = send(m, press("right"))
	before := rows(m)

	m = send(m, press("f"))
	m = send(m, press("f"))
	if h := header(m); h != "~/repo · all agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m, before...)

	// collapsing inside f doesn't carry over to the next time f is turned on
	m = send(m, press("f"))
	m = send(m, press("g"))
	m = send(m, press("left"))
	m = send(m, press("f"))
	m = send(m, press("f"))
	if got := rows(m)[0]; got != "> droid 1h ago 1 plan −3 sub" {
		t.Errorf("first row = %q, want plan expanded again", got)
	}
}

func TestExpansionAndFSurviveScopeAndAgentSwitches(t *testing.T) {
	m := send(start(family()), press("right")) // plan open
	m = send(m, press("s"))
	if got := rows(m)[0]; got != "> droid 1h ago 1 plan −3 sub ." {
		t.Errorf("after s, first row = %q", got)
	}
	m = send(m, press("tab")) // codex
	m = send(m, press("tab")) // droid
	if got := rows(m)[0]; got != "> droid 1h ago 1 plan −3 sub ." {
		t.Errorf("after tab, first row = %q", got)
	}

	m = send(m, press("f"))
	m = send(m, press("s"))
	m = send(m, press("tab")) // claude
	if h := header(m); h != "all directories · claude · with sub-agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m,
		"> claude 6h ago 1 ship −1 sub ~/repo",
		"└─ claude 7h ago 1 lint")
}

func TestTextFilterSurvivesF(t *testing.T) {
	m := start(family())
	m = send(m, press("/"))
	m = send(m, press("ship"))
	m = send(m, press("enter"))
	assertRows(t, m, "> claude 6h ago 1 ship +1 sub")
	m = send(m, press("f"))
	assertRows(t, m, "> claude 6h ago 1 ship −1 sub")
}

func TestExpandingReappliesTextFilter(t *testing.T) {
	m := start(family())
	m = send(m, press("/"))
	m = send(m, press("claude"))
	m = send(m, press("enter"))
	m = send(m, press("right"))
	assertRows(t, m,
		"> claude 6h ago 1 ship −1 sub",
		"└─ claude 7h ago 1 lint")
}

func TestExpandedStartShowsEveryTreeAndOrphans(t *testing.T) {
	cfg := family()
	cfg.Expanded = true
	assertRows(t, start(cfg),
		"> droid 1h ago 1 plan −3 sub",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship −1 sub",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan)")
}

func TestFWithNothingToShow(t *testing.T) {
	m := send(start(project(index.Here)), press("f"))
	if !strings.Contains(m.View(), "No sessions with sub-agents here · s widen scope · f show all") {
		t.Errorf("no hint:\n%s", m.View())
	}
	m = send(start(project(index.All)), press("f"))
	if v := m.View(); !strings.Contains(v, "No sessions with sub-agents anywhere · f show all") || strings.Contains(v, "widen") {
		t.Errorf("unexpected hint:\n%s", v)
	}
}

func numbered(n int) Config {
	cfg := project(index.Here)
	cfg.Sessions = nil
	for i := range n {
		cfg.Sessions = append(cfg.Sessions, at("droid", fmt.Sprintf("d%02d", i), fmt.Sprintf("t%02d", i), "/repo", i+1))
	}
	return cfg
}

func firstTitle(t *testing.T, m tea.Model) string {
	t.Helper()
	r := rows(m)
	if len(r) == 0 {
		t.Fatal("no rows")
	}
	f := strings.Fields(r[0])
	return f[len(f)-1]
}

func TestPagingKeys(t *testing.T) {
	m := start(numbered(80)) // 30 rows tall: several pages
	page1 := firstTitle(t, m)
	for _, k := range []string{"pgdown", "d", "ctrl+f"} {
		next := send(m, press(k))
		if firstTitle(t, next) == page1 {
			t.Errorf("%s did not go to the next page", k)
		}
		for _, back := range []string{"pgup", "u", "b", "ctrl+b"} {
			if got := firstTitle(t, send(next, press(back))); got != page1 {
				t.Errorf("%s then %s: first row %s, want %s", k, back, got, page1)
			}
		}
	}
	for _, k := range []string{"right", "l"} {
		if got := firstTitle(t, send(m, press(k))); got != page1 {
			t.Errorf("%s changed page", k)
		}
	}
	page2 := send(m, press("pgdown"))
	for _, k := range []string{"left", "h"} {
		if got, want := firstTitle(t, send(page2, press(k))), firstTitle(t, page2); got != want {
			t.Errorf("%s changed page: %s, want %s", k, got, want)
		}
	}
	if v := send(page2, press("f")).View(); !strings.Contains(v, "No sessions with sub-agents") {
		t.Errorf("f did not switch to the sub-agent view:\n%s", v)
	}

	if !strings.Contains(m.View(), "→ expand • f subagents • q quit • ? more") {
		t.Errorf("short help lacks the new keys:\n%s", m.View())
	}
	help := send(m, press("?")).View()
	for _, want := range []string{"pgdn/d", "pgup/u", "collapse"} {
		if !strings.Contains(help, want) {
			t.Errorf("full help lacks %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "→/l") || strings.Contains(help, "←/h") {
		t.Errorf("full help still offers arrows for paging:\n%s", help)
	}
}

func TestHeaderStaysVisibleWhenExpandingGrowsList(t *testing.T) {
	const height = 45
	cfg := numbered(1)
	cfg.Sessions[0].Children = numbered(200).Sessions
	m := send(newModel(cfg), tea.WindowSizeMsg{Width: 140, Height: height})
	m = send(m, press("right"))
	lines := strings.Split(m.View(), "\n")
	if len(lines) > height {
		t.Errorf("view has %d lines, window has %d", len(lines), height)
	}
	if !strings.Contains(lines[0], "~/repo · all agents") {
		t.Errorf("first line = %q, want the header", strings.TrimSpace(lines[0]))
	}
}

func TestReexpandingOpensOneLevelAgain(t *testing.T) {
	m := start(family())
	for _, k := range []string{"right", "j", "right", "k", "left", "right"} {
		m = send(m, press(k))
	}
	assertRows(t, m,
		"> droid 1h ago 1 plan −3 sub",
		"├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo",
		"claude 6h ago 1 ship +1 sub")
}

// line returns the unstyled view line containing s.
func line(t *testing.T, m tea.Model, s string) string {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(m.View()), "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	t.Fatalf("no line with %q in:\n%s", s, m.View())
	return ""
}

func TestSubagentRowsAreIndentedAsAWhole(t *testing.T) {
	cfg := family()
	cfg.Expanded = true
	cfg.Sessions[0].Children[1].Problem = "unreadable" // docs
	m := start(cfg)

	plan, build, probe := line(t, m, "plan"), line(t, m, "build"), line(t, m, "probe")
	for l, prefix := range map[string]string{
		plan:                "> droid ",
		build:               "  ├─ droid ",
		probe:               "  │  └─ droid ",
		line(t, m, "docs"):  "  └─ droid ",
		line(t, m, "lint"):  "  └─ claude ",
		line(t, m, "stray"): "  droid ",
		line(t, m, "solo"):  "  codex ",
		line(t, m, "ship"):  "  claude ",
	} {
		if !strings.HasPrefix(l, prefix) {
			t.Errorf("line %q does not start with %q", l, prefix)
		}
	}
	col := func(l, s string) int { return ansi.StringWidth(l[:strings.Index(l, s)]) }
	if got := col(build, "2h ago") - col(plan, "1h ago"); got != 3 {
		t.Errorf("child columns shifted by %d, want 3", got)
	}
	if got := col(probe, "3h ago") - col(plan, "1h ago"); got != 6 {
		t.Errorf("grandchild columns shifted by %d, want 6", got)
	}
}

func TestSubagentCwdShownOnlyWhenItDiffersFromParent(t *testing.T) {
	cfg := family()
	cfg.Expanded = true
	cfg.Scope = index.All
	cfg.Sessions[0].Children[0].Children[0].Cwd = home + "/repo/web" // probe
	assertRows(t, start(cfg),
		"> droid 1h ago 1 plan −3 sub ~/repo",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe ~/repo/web",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo ~/repo",
		"claude 6h ago 1 ship −1 sub ~/repo",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) ~/repo")
}

func TestRightOnLeafDoesNothing(t *testing.T) {
	m := send(start(family()), press("j"))
	before := rows(m)
	m = send(m, press("right"))
	assertRows(t, m, before...)
}
