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
		"> droid 1h ago 1 plan +3 sub .",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")
}

func TestRightExpandsOneLevelAtATime(t *testing.T) {
	m := send(start(family()), press("right"))
	assertRows(t, m,
		"> droid 1h ago 1 plan −3 sub .",
		"├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")

	m = send(m, press("j"))
	m = send(m, press("l"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub .",
		"> ├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")
}

func TestLeftCollapses(t *testing.T) {
	m := start(family())
	m = send(m, press("right"))
	m = send(m, press("j"))
	m = send(m, press("right")) // plan and build are open, cursor on build

	m = send(m, press("left"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub .",
		"> ├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")

	// on a sub-agent row, ← goes to the parent and closes it
	m = send(m, press("h"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub .",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")

	// on a closed top-level row, ← does nothing
	m = send(m, press("left"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub .",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")
}

func TestCollapsingFromGrandchildClosesItsParentOnly(t *testing.T) {
	m := start(family())
	for _, k := range []string{"right", "j", "right", "j"} { // cursor on probe
		m = send(m, press(k))
	}
	m = send(m, press("left"))
	assertRows(t, m,
		"droid 1h ago 1 plan −3 sub .",
		"> ├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")
}

func TestFShowsOnlySessionsWithSubagentsExpanded(t *testing.T) {
	m := send(start(family()), press("f"))
	if h := header(m); h != "~/repo · all agents · with sub-agents" {
		t.Errorf("header = %q", h)
	}
	assertRows(t, m,
		"> droid 1h ago 1 plan −3 sub .",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"claude 6h ago 1 ship −1 sub .",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) .")

	// ← and → still work inside f
	m = send(m, press("left"))
	assertRows(t, m,
		"> droid 1h ago 1 plan +3 sub .",
		"claude 6h ago 1 ship −1 sub .",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) .")
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
	if got := rows(m)[0]; got != "> droid 1h ago 1 plan −3 sub ." {
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
	assertRows(t, m, "> claude 6h ago 1 ship +1 sub .")
	m = send(m, press("f"))
	assertRows(t, m, "> claude 6h ago 1 ship −1 sub .")
}

func TestExpandingReappliesTextFilter(t *testing.T) {
	m := start(family())
	m = send(m, press("/"))
	m = send(m, press("claude"))
	m = send(m, press("enter"))
	m = send(m, press("right"))
	assertRows(t, m,
		"> claude 6h ago 1 ship −1 sub .",
		"└─ claude 7h ago 1 lint")
}

func TestExpandedStartShowsEveryTreeAndOrphans(t *testing.T) {
	cfg := family()
	cfg.Expanded = true
	assertRows(t, start(cfg),
		"> droid 1h ago 1 plan −3 sub .",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship −1 sub .",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) .")
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

func firstRow(t *testing.T, m tea.Model) string {
	t.Helper()
	r := rows(m)
	if len(r) == 0 {
		t.Fatal("no rows")
	}
	return r[0]
}

func TestPagingKeys(t *testing.T) {
	m := start(numbered(80)) // 30 rows tall: several pages
	page1 := firstRow(t, m)
	for _, k := range []string{"pgdown", "d", "ctrl+f"} {
		next := send(m, press(k))
		if firstRow(t, next) == page1 {
			t.Errorf("%s did not go to the next page", k)
		}
		for _, back := range []string{"pgup", "u", "b", "ctrl+b"} {
			if got := firstRow(t, send(next, press(back))); got != page1 {
				t.Errorf("%s then %s: first row %s, want %s", k, back, got, page1)
			}
		}
	}
	for _, k := range []string{"right", "l"} {
		if got := firstRow(t, send(m, press(k))); got != page1 {
			t.Errorf("%s changed page", k)
		}
	}
	page2 := send(m, press("pgdown"))
	for _, k := range []string{"left", "h"} {
		if got, want := firstRow(t, send(page2, press(k))), firstRow(t, page2); got != want {
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
		"> droid 1h ago 1 plan −3 sub .",
		"├─ droid 2h ago 1 build +1 sub",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship +1 sub .")
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
	scoped := []string{
		"> droid 1h ago 1 plan −3 sub .",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe ./web",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo .",
		"claude 6h ago 1 ship −1 sub .",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) .",
	}
	global := []string{
		"> droid 1h ago 1 plan −3 sub ~/repo",
		"├─ droid 2h ago 1 build −1 sub",
		"│ └─ droid 3h ago 1 probe ~/repo/web",
		"└─ droid 4h ago 1 docs",
		"codex 5h ago 1 solo ~/repo",
		"claude 6h ago 1 ship −1 sub ~/repo",
		"└─ claude 7h ago 1 lint",
		"droid 8h ago 1 stray (orphan) ~/repo",
	}
	for _, tc := range []struct {
		scope index.Scope
		want  []string
	}{{index.Here, scoped}, {index.Subdirs, scoped}, {index.All, global}} {
		cfg := family()
		cfg.Expanded = true
		cfg.Scope = tc.scope
		cfg.Sessions[0].Children[0].Children[0].Cwd = home + "/repo/web" // probe
		assertRows(t, start(cfg), tc.want...)
	}
}

func TestRightOnLeafDoesNothing(t *testing.T) {
	m := send(start(family()), press("j"))
	before := rows(m)
	m = send(m, press("right"))
	assertRows(t, m, before...)
}

func TestSlugShownInAllScope(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoSlug = "org/repo"
	cfg := Config{
		Sessions: []*agent.Session{s},
		Dir:      home + "/repo",
		Scope:    index.All,
		Home:     home,
		Agents:   []string{"droid"},
	}
	m := start(cfg)
	assertRows(t, m, "> droid 1h ago 1 alpha org/repo ~/repo")
}

func TestSlugShownInHereScope(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoSlug = "org/repo"
	cfg := Config{
		Sessions: []*agent.Session{s},
		Dir:      home + "/repo",
		Scope:    index.Here,
		Home:     home,
		Agents:   []string{"droid"},
	}
	m := start(cfg)
	assertRows(t, m, "> droid 1h ago 1 alpha org/repo .")
}

func TestSlugHiddenOnSubagentWhenMatchingParent(t *testing.T) {
	newConfig := func(scope index.Scope) Config {
		parent := at("droid", "plan", "plan", "/repo", 1)
		parent.RepoSlug = "org/repo"
		child := at("droid", "build", "build", "/repo", 2)
		child.RepoSlug = "org/repo"
		different := at("droid", "docs", "docs", "/repo", 3)
		different.RepoSlug = "other/lib"
		parent.Children = []*agent.Session{child, different}
		return Config{
			Sessions: []*agent.Session{parent},
			Dir:      home + "/repo",
			Scope:    scope,
			Home:     home,
			Agents:   []string{"droid"},
			Expanded: true,
		}
	}
	for _, tc := range []struct {
		scope index.Scope
		cwd   string
	}{{index.Here, "."}, {index.Subdirs, "."}, {index.All, "~/repo"}} {
		assertRows(t, start(newConfig(tc.scope)),
			"> droid 1h ago 1 plan −2 sub org/repo "+tc.cwd,
			"├─ droid 2h ago 1 build",
			"└─ droid 3h ago 1 docs other/lib")
	}
}

// PRD 0005 story 4: a sub-agent that ran elsewhere keeps its identity in every
// scope — outside the starting directory as a home-shortened path, below it as
// a relative one, with its own slug when the repository differs.
func TestSubagentOutsideStartingDirShownInEveryScope(t *testing.T) {
	for _, tc := range []struct {
		scope index.Scope
		want  []string
	}{
		{index.Here, []string{
			"> droid 1h ago 1 plan −2 sub org/repo .",
			"├─ droid 2h ago 1 build other/lib ~/other",
			"└─ droid 3h ago 1 docs ./web",
		}},
		{index.Subdirs, []string{
			"> droid 1h ago 1 plan −2 sub org/repo .",
			"├─ droid 2h ago 1 build other/lib ~/other",
			"└─ droid 3h ago 1 docs ./web",
		}},
		{index.All, []string{
			"> droid 1h ago 1 plan −2 sub org/repo ~/repo",
			"├─ droid 2h ago 1 build other/lib ~/other",
			"└─ droid 3h ago 1 docs ~/repo/web",
		}},
	} {
		parent := at("droid", "plan", "plan", "/repo", 1)
		parent.RepoSlug = "org/repo"
		outside := at("droid", "build", "build", "/other", 2)
		outside.RepoSlug = "other/lib"
		below := at("droid", "docs", "docs", "/repo/web", 3)
		below.RepoSlug = "org/repo"
		parent.Children = []*agent.Session{outside, below}
		cfg := Config{
			Sessions: []*agent.Session{parent},
			Dir:      home + "/repo",
			Scope:    tc.scope,
			Home:     home,
			Agents:   []string{"droid"},
			Expanded: true,
		}
		assertRows(t, start(cfg), tc.want...)
	}
}

// PRD 0005 story 2: pressing s never changes a row's slot layout — the slug
// and a cwd are on screen in every scope, only the cwd's form changes.
func TestRowSlotsStableAcrossScopeSwitches(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoSlug = "org/repo"
	cfg := Config{
		Sessions: []*agent.Session{s},
		Dir:      home + "/repo",
		Scope:    index.Here,
		Home:     home,
		Agents:   []string{"droid"},
	}
	m := start(cfg)
	assertRows(t, m, "> droid 1h ago 1 alpha org/repo .")
	m = send(m, press("s"))
	assertRows(t, m, "> droid 1h ago 1 alpha org/repo .")
	m = send(m, press("s"))
	assertRows(t, m, "> droid 1h ago 1 alpha org/repo ~/repo")
}

func TestDirNameFallbackShownInAllScope(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoDirName = "myproject"
	cfg := Config{
		Sessions: []*agent.Session{s},
		Dir:      home + "/repo",
		Scope:    index.All,
		Home:     home,
		Agents:   []string{"droid"},
	}
	m := start(cfg)
	assertRows(t, m, "> droid 1h ago 1 alpha myproject ~/repo")
}

func TestSlugDroppedBeforeCwdUnderWidthPressure(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoSlug = "org/repo"
	cfg := Config{
		Sessions: []*agent.Session{s},
		Dir:      home + "/repo",
		Scope:    index.All,
		Home:     home,
		Agents:   []string{"droid"},
	}
	// Wide: both slug and cwd visible
	wide := start(cfg)
	wr := rows(wide)
	if len(wr) == 0 {
		t.Fatal("no rows at wide width")
	}
	if !strings.Contains(wr[0], "org/repo") || !strings.Contains(wr[0], "~/repo") {
		t.Errorf("wide: expected both slug and cwd: %q", wr[0])
	}
	// Narrow: slug dropped, cwd survives
	narrow := send(newModel(cfg), tea.WindowSizeMsg{Width: 44, Height: 30})
	nr := rows(narrow)
	if len(nr) == 0 {
		t.Fatal("no rows at narrow width")
	}
	if strings.Contains(nr[0], "org/repo") {
		t.Errorf("narrow: slug should be dropped: %q", nr[0])
	}
}

func TestSlugInFilterValueInAllScopes(t *testing.T) {
	s := at("droid", "d1", "alpha", "/repo", 1)
	s.RepoSlug = "org/repo"

	for _, scope := range []index.Scope{index.Here, index.Subdirs, index.All} {
		cfg := Config{
			Sessions: []*agent.Session{s},
			Dir:      home + "/repo",
			Scope:    scope,
			Home:     home,
			Agents:   []string{"droid"},
		}
		m := start(cfg)
		items := m.(model).list.Items()
		if len(items) == 0 {
			t.Fatalf("scope %d: no items", scope)
		}
		if fv := items[0].FilterValue(); !strings.Contains(fv, "org/repo") {
			t.Errorf("scope %d: FilterValue = %q, should contain slug", scope, fv)
		}
	}
}
