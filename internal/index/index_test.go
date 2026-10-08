package index_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/index"
)

func TestDroidSessionInCurrentDirectory(t *testing.T) {
	f := newFixture(t)
	proj := f.project("Developer/app")
	other := f.project("Developer/other")

	f.droidSession(proj, "d-titled", 2, obj{"title": "Fix login bug"},
		droidUser("the login page crashes"), droidAssistant("looking"), droidUser("thanks"))
	f.droidSession(proj, "d-untitled", 3, nil,
		droidUser("add a dark mode\nplease"), droidAssistant("ok"))
	f.droidSession(other, "d-elsewhere", 4, obj{"title": "Not here"}, droidUser("hi"))

	got := f.list(f.options(proj))

	assertList(t, got, `
		droid d-untitled "add a dark mode please" turns=1
		droid d-titled "Fix login bug" turns=2
	`)
	want := &agent.Command{Bin: "droid", Args: []string{"--resume", "d-untitled"}, Dir: proj}
	if !reflect.DeepEqual(got[0].Resume, want) {
		t.Errorf("resume = %+v, want %+v", got[0].Resume, want)
	}
}

func TestClaudeSessions(t *testing.T) {
	f := newFixture(t)
	// '.' and '_' are encoded as '-' too; matching must still work
	proj := f.project("Developer/my_app.v2")

	f.claudeSession(proj, "c-titled", 5,
		claudeUser("<command-name>/clear</command-name>"),
		claudeUser("refactor the parser"),
		claudeAssistant("done"),
		obj{"type": "ai-title", "aiTitle": "Old title", "sessionId": "c-titled"},
		claudeUser([]obj{{"type": "tool_result", "content": "x"}}),
		claudeUser([]obj{{"type": "text", "text": "now add tests"}}),
		obj{"type": "ai-title", "aiTitle": "Parser refactor", "sessionId": "c-titled"},
	)
	f.claudeSession(proj, "c-plain", 4, claudeUser("why is CI red"))
	meta := claudeUser("hidden meta prompt")
	meta["isMeta"] = true
	f.claudeSession(proj, "c-meta-only", 3, meta)

	got := f.list(f.options(proj))
	assertList(t, got, `
		claude c-titled "Parser refactor" turns=2
		claude c-plain "why is CI red" turns=1
	`)
	want := &agent.Command{Bin: "claude", Args: []string{"--resume", "c-titled"}, Dir: proj}
	if !reflect.DeepEqual(got[0].Resume, want) {
		t.Errorf("resume = %+v, want %+v", got[0].Resume, want)
	}
}

func TestCodexSessions(t *testing.T) {
	f := newFixture(t)
	proj := f.project("work/api")
	other := f.project("work/web")

	f.codexSession(proj, "x-named", 6, nil, "set up the db", "add migrations")
	f.codexSession(proj, "x-desktop", 5, obj{"originator": "Codex Desktop", "source": "vscode"}, "from the desktop app")
	f.codexSession(proj, "x-empty", 4, nil)
	f.codexSession(other, "x-other", 7, nil, "elsewhere")
	f.codexIndex(
		obj{"id": "x-named", "thread_name": "First name", "updated_at": "2026-09-01T00:00:00Z"},
		obj{"id": "x-named", "thread_name": "Database setup", "updated_at": "2026-09-01T01:00:00Z"},
	)

	got := f.list(f.options(proj))
	assertList(t, got, `
		codex x-named "Database setup" turns=2
		codex x-desktop "from the desktop app" turns=1
	`)
	want := &agent.Command{Bin: "codex", Args: []string{"resume", "x-named"}, Dir: proj}
	if len(got) > 0 && !reflect.DeepEqual(got[0].Resume, want) {
		t.Errorf("resume = %+v, want %+v", got[0].Resume, want)
	}
}

func TestPiSessions(t *testing.T) {
	f := newFixture(t)
	proj := f.project("code/tool")

	named := f.piSession(proj, "p-named", 3, nil,
		piUser("write a readme"),
		obj{"type": "message", "message": obj{"role": "toolResult", "content": []obj{{"type": "text", "text": "ok"}}}},
		piName("first"), piName("Readme draft"),
		piUser("<skill>injected</skill>"),
		piUser("shorter please"))
	f.piSession(proj, "p-plain", 2, nil, piUser("what does main.go do"))

	got := f.list(f.options(proj))
	assertList(t, got, `
		pi p-named "Readme draft" turns=2
		pi p-plain "what does main.go do" turns=1
	`)
	want := &agent.Command{Bin: "pi", Args: []string{"--session", named}, Dir: proj}
	if len(got) > 0 && !reflect.DeepEqual(got[0].Resume, want) {
		t.Errorf("resume = %+v, want %+v", got[0].Resume, want)
	}
}

// subagentFixture builds one parent per agent, each with sub-agents, plus a
// nested droid chain and an orphan.
func subagentFixture(t *testing.T) (*fixture, string) {
	f := newFixture(t)
	proj := f.project("proj")

	// droid: parent -> child -> grandchild, plus an orphan
	f.droidSession(proj, "d-parent", 40, obj{"title": "Droid main"}, droidUser("plan it"))
	f.droidSession(proj, "d-child", 41, obj{"title": "Worker: build", "callingSessionId": "d-parent"}, droidUser("build it"))
	f.droidSession(proj, "d-grandchild", 42, obj{"title": "Explorer: look", "callingSessionId": "d-child"}, droidUser("look"))
	f.droidSession(proj, "d-orphan", 10, obj{"title": "Lost worker", "callingSessionId": "gone"}, droidUser("x"))

	// claude: two sub-agents under one parent
	f.claudeSession(proj, "c-parent", 30, claudeUser("big task"))
	f.claudeSubagent(proj, "c-parent", "a1", 31, obj{"agentType": "general-purpose", "description": "Scan repo"}, claudeUser("scan the repo"))
	f.claudeSubagent(proj, "c-parent", "a2", 32, obj{"agentType": "Explore", "description": "Find tests"}, claudeUser("find tests"))

	// codex: review sub-agent with object-valued source
	f.codexSession(proj, "x-parent", 20, nil, "ship it")
	f.codexSession(proj, "x-review", 21, obj{"thread_source": "subagent", "parent_thread_id": "x-parent", "source": obj{"subagent": "review"}}, "review the diff")
	f.codexSession(proj, "x-guard", 22, obj{"thread_source": "subagent", "parent_thread_id": "x-parent", "source": obj{"subagent": obj{"other": "guardian"}}}, "check safety")

	// pi: sub-agent pointing at the parent file
	parent := f.piSession(proj, "p-parent", 25, nil, piUser("main pi task"))
	f.piSession(proj, "p-sub", 26, obj{"parentSession": parent}, piName("chapter-extractor#0a1b2c3d"), piUser("extract chapter 1"))
	return f, proj
}

func TestSubagentsHiddenByDefault(t *testing.T) {
	f, proj := subagentFixture(t)
	assertList(t, f.list(f.options(proj)), `
		droid d-parent "Droid main" turns=1 +2 sub
		claude c-parent "big task" turns=1 +2 sub
		pi p-parent "main pi task" turns=1 +1 sub
		codex x-parent "ship it" turns=1 +2 sub
	`)
}

func TestSubagentsShownAsTree(t *testing.T) {
	f, proj := subagentFixture(t)
	opts := f.options(proj)
	opts.Subagents = true
	got := f.list(opts)
	assertList(t, got, `
		droid d-parent "Droid main" turns=1
		  droid d-child "Worker: build" turns=1
		    droid d-grandchild "Explorer: look" turns=1
		claude c-parent "big task" turns=1
		  claude a2 "Find tests" turns=1 tag=Explore
		  claude a1 "Scan repo" turns=1 tag=general-purpose
		pi p-parent "main pi task" turns=1
		  pi p-sub "extract chapter 1" turns=1 tag=chapter-extractor
		codex x-parent "ship it" turns=1
		  codex x-guard "check safety" turns=1 tag=guardian
		  codex x-review "review the diff" turns=1 tag=review
		droid d-orphan "Lost worker" turns=1 orphan
	`)

	byID := map[string]*agent.Session{}
	var walk func([]*agent.Session)
	walk = func(ss []*agent.Session) {
		for _, s := range ss {
			byID[s.ID] = s
			walk(s.Children)
		}
	}
	walk(got)
	resumes := map[string][]string{
		"a1":      {"--resume", "c-parent"}, // claude sub-agents resume their parent
		"x-guard": {"resume", "x-guard"},
		"d-child": {"--resume", "d-child"},
	}
	for id, want := range resumes {
		if s := byID[id]; s == nil || !reflect.DeepEqual(s.Resume.Args, want) {
			t.Errorf("%s resume args = %v, want %v", id, s.Resume.Args, want)
		}
	}
}

func TestHeadlessSessionsHiddenUnlessRequested(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	sdk := claudeUser("automated prompt")
	sdk["entrypoint"] = "sdk-ts"
	f.claudeSession(proj, "c-sdk", 3, sdk)
	f.claudeSession(proj, "c-cli", 2, claudeUser("typed by me"))
	f.codexSession(proj, "x-sdk", 4, obj{"originator": "repoprompt"}, "tool call")
	// a sub-agent of a hidden headless parent has no visible parent
	f.codexSession(proj, "x-sdk-sub", 5, obj{"originator": "codex-tui", "thread_source": "subagent", "parent_thread_id": "x-sdk"}, "sub of sdk")

	opts := f.options(proj)
	assertList(t, f.list(opts), `
		claude c-cli "typed by me" turns=1
	`)

	opts.Subagents = true
	assertList(t, f.list(opts), `
		codex x-sdk-sub "sub of sdk" turns=1 orphan
		claude c-cli "typed by me" turns=1
	`)

	opts.Headless = true
	assertList(t, f.list(opts), `
		codex x-sdk "tool call" turns=1
		  codex x-sdk-sub "sub of sdk" turns=1
		claude c-sdk "automated prompt" turns=1
		claude c-cli "typed by me" turns=1
	`)
}

func TestAgentFilter(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	f.claudeSession(proj, "c1", 1, claudeUser("a"))
	f.codexSession(proj, "x1", 2, nil, "b")
	f.droidSession(proj, "d1", 3, nil, droidUser("c"))

	opts := f.options(proj)
	opts.Agents = []string{"claude", "droid"}
	assertList(t, f.list(opts), `
		droid d1 "c" turns=1
		claude c1 "a" turns=1
	`)
}

func TestGlobalAndRecursiveScopes(t *testing.T) {
	f := newFixture(t)
	root := f.project("repo")
	sub := f.project("repo/web")
	sibling := f.project("repo-old") // shares the encoded prefix but is not a subdirectory
	elsewhere := f.project("other")
	f.droidSession(root, "d-root", 1, nil, droidUser("root"))
	f.claudeSession(sub, "c-sub", 2, claudeUser("sub"))
	f.piSession(sibling, "p-sibling", 3, nil, piUser("sibling"))
	f.codexSession(elsewhere, "x-else", 4, nil, "else")

	opts := f.options(root)
	assertList(t, f.list(opts), `
		droid d-root "root" turns=1
	`)

	opts.Scope = index.Subdirs
	assertList(t, f.list(opts), `
		claude c-sub "sub" turns=1
		droid d-root "root" turns=1
	`)

	opts.Scope = index.All
	assertList(t, f.list(opts), `
		codex x-else "else" turns=1
		pi p-sibling "sibling" turns=1
		claude c-sub "sub" turns=1
		droid d-root "root" turns=1
	`)
}

func TestSymlinkedDirectoryMatches(t *testing.T) {
	f := newFixture(t)
	real := f.project("real/app")
	link := filepath.Join(f.home, "link-app")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// Agents record the physical path (getcwd), while the shell's $PWD (and so
	// the tool's cwd) may be the symlink.
	f.droidSession(real, "d-real", 1, nil, droidUser("droid"))
	f.claudeSession(real, "c-real", 2, claudeUser("claude"))

	want := `
		claude c-real "claude" turns=1
		droid d-real "droid" turns=1
	`
	assertList(t, f.list(f.options(link)), want)
	assertList(t, f.list(f.options(real)), want)
}

func TestSessionRecordedUnderSymlinkMatchesRealPath(t *testing.T) {
	f := newFixture(t)
	real := f.project("real/app")
	link := filepath.Join(f.home, "link-app")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	f.droidSession(link, "d-link", 1, nil, droidUser("droid"))
	f.piSession(link, "p-link", 2, nil, piUser("pi"))

	assertList(t, f.list(f.options(real)), `
		pi p-link "pi" turns=1
		droid d-link "droid" turns=1
	`)
}

func TestSubagentInAnotherDirectoryStaysUnderParent(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	sub := f.project("proj/pkg")
	parent := f.piSession(proj, "p-parent", 1, nil, piUser("main"))
	f.piSession(sub, "p-sub", 2, obj{"parentSession": parent}, piUser("helper"))
	f.droidSession(proj, "d-parent", 3, nil, droidUser("main"))
	f.droidSession(sub, "d-sub", 4, obj{"callingSessionId": "d-parent"}, droidUser("helper"))

	assertList(t, f.list(f.options(proj)), `
		droid d-parent "main" turns=1 +1 sub
		pi p-parent "main" turns=1 +1 sub
	`)
}

func TestUnresumableSessionsAreMarked(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	gone := filepath.Join(f.home, "deleted-project")
	f.bins["pi"] = false
	f.piSession(proj, "p1", 2, nil, piUser("pi not installed"))
	f.droidSession(gone, "d-gone", 1, nil, droidUser("dir removed"))

	opts := f.options(proj)
	opts.Scope = index.All
	assertList(t, f.list(opts), `
		pi p1 "pi not installed" turns=1 problem="pi is not installed"
		droid d-gone "dir removed" turns=1 problem="directory no longer exists: `+gone+`"
	`)
}

func TestMalformedFilesAreSkippedAndReported(t *testing.T) {
	f := newFixture(t)
	proj := f.project("proj")
	f.droidSession(proj, "d-ok", 2, nil, droidUser("fine"))
	// a session still being written ends in a partial line
	f.droidSession(proj, "d-partial", 3, nil, droidUser("in progress"), `{"type":"message","mess`)
	f.piSession(proj, "p-partial", 4, nil, piUser("pi in progress"), `{"type":`)
	bad := filepath.Join(f.home, ".factory", "sessions", droidDir(proj), "broken.jsonl")
	f.write(bad, 1, "{not json")

	res, err := index.List(f.options(proj))
	if err != nil {
		t.Fatal(err)
	}
	assertList(t, res.Sessions, `
		pi p-partial "pi in progress" turns=1
		droid d-partial "in progress" turns=1
		droid d-ok "fine" turns=1
	`)
	if len(res.Skipped) != 1 || res.Skipped[0].Path != bad {
		t.Errorf("skipped = %+v, want just %s", res.Skipped, bad)
	}
}

func TestDroidIgnoresInjectedMessagesAndHidesEmptySessions(t *testing.T) {
	f := newFixture(t)
	proj := f.project("app")

	hook := obj{"type": "message", "message": obj{"role": "user", "content": []obj{}, "visibility": "user_only", "hookEventName": "SessionStart"}}
	reminder := obj{"type": "message", "message": obj{"role": "user", "visibility": "llm_only", "content": []obj{{"type": "text", "text": "<system-reminder>x</system-reminder>"}}}}
	toolResult := obj{"type": "message", "message": obj{"role": "user", "content": []obj{{"type": "tool_result", "content": "ok"}}}}
	mixed := obj{"type": "message", "message": obj{"role": "user", "content": []obj{
		{"type": "text", "text": "<system-reminder>ctx</system-reminder>"},
		{"type": "text", "text": "real question"},
	}}}

	f.droidSession(proj, "d-empty", 1, nil, hook, reminder)
	f.droidSession(proj, "d-real", 2, nil, hook, reminder, mixed, toolResult)

	assertList(t, f.list(f.options(proj)), `
		droid d-real "real question" turns=1
	`)
}
