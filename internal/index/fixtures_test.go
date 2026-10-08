package index_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/index"
)

// fixture is a fake home directory populated with synthetic session files that
// mirror each agent's real on-disk format.
type fixture struct {
	t    *testing.T
	home string
	env  map[string]string
	// installed binaries; anything else is reported as not installed
	bins map[string]bool
	base time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{
		t:    t,
		home: home,
		env:  map[string]string{},
		bins: map[string]bool{"pi": true, "claude": true, "codex": true, "droid": true},
		base: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
	}
}

// project creates a real directory under the fake home and returns its path.
func (f *fixture) project(rel string) string {
	f.t.Helper()
	p := filepath.Join(f.home, rel)
	if err := os.MkdirAll(p, 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// write creates a jsonl file from the given lines and sets its mtime to
// base + age offset hours, so ordering is deterministic.
func (f *fixture) write(path string, hour int, lines ...any) string {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	var b strings.Builder
	for _, l := range lines {
		switch v := l.(type) {
		case string:
			b.WriteString(v)
		default:
			data, err := json.Marshal(v)
			if err != nil {
				f.t.Fatal(err)
			}
			b.Write(data)
		}
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.touch(path, hour)
	return path
}

func (f *fixture) touch(path string, hour int) {
	f.t.Helper()
	ts := f.base.Add(time.Duration(hour) * time.Hour)
	if err := os.Chtimes(path, ts, ts); err != nil {
		f.t.Fatal(err)
	}
}

type obj = map[string]any

func (f *fixture) options(cwd string) index.Options {
	return index.Options{
		Env: agent.Env{Home: f.home, Getenv: func(k string) string { return f.env[k] }},
		Cwd: cwd,
		LookPath: func(bin string) (string, error) {
			if f.bins[bin] {
				return "/usr/bin/" + bin, nil
			}
			return "", fmt.Errorf("%s: not found", bin)
		},
	}
}

func (f *fixture) list(opts index.Options) []*agent.Session {
	f.t.Helper()
	res, err := index.List(opts)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, s := range res.Skipped {
		f.t.Logf("skipped %s: %v", s.Path, s.Err)
	}
	return res.Sessions
}

// render prints the forest one session per line so expectations read like the
// list a user would see.
func render(sessions []*agent.Session) string {
	var b strings.Builder
	var walk func(ss []*agent.Session, depth int)
	walk = func(ss []*agent.Session, depth int) {
		for _, s := range ss {
			b.WriteString(strings.Repeat("  ", depth))
			fmt.Fprintf(&b, "%s %s %q turns=%d", s.Agent, s.ID, s.DisplayTitle(), s.Turns)
			if s.Tag != "" {
				fmt.Fprintf(&b, " tag=%s", s.Tag)
			}
			if s.HiddenSubagents > 0 {
				fmt.Fprintf(&b, " +%d sub", s.HiddenSubagents)
			}
			if s.Orphan {
				b.WriteString(" orphan")
			}
			if s.Problem != "" {
				fmt.Fprintf(&b, " problem=%q", s.Problem)
			}
			b.WriteByte('\n')
			walk(s.Children, depth+1)
		}
	}
	walk(sessions, 0)
	return b.String()
}

func assertList(t *testing.T, got []*agent.Session, want string) {
	t.Helper()
	want = strings.TrimLeft(dedent(want), "\n")
	if g := render(got); g != want {
		t.Errorf("listing mismatch\n--- got ---\n%s--- want ---\n%s", g, want)
	}
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, "\t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if len(l) >= indent && indent > 0 {
			lines[i] = l[indent:]
		}
	}
	out := strings.Join(lines, "\n")
	return strings.TrimRight(out, "\t ")
}

// ---- per-agent writers (formats observed in real session files) ----

func droidDir(cwd string) string { return strings.ReplaceAll(cwd, "/", "-") }

func (f *fixture) droidSession(cwd, id string, hour int, start obj, lines ...any) string {
	head := obj{"type": "session_start", "id": id, "title": "New Session", "owner": "me", "version": 2, "cwd": cwd}
	for k, v := range start {
		head[k] = v
	}
	path := filepath.Join(f.home, ".factory", "sessions", droidDir(cwd), id+".jsonl")
	return f.write(path, hour, append([]any{head}, lines...)...)
}

func droidUser(text string) obj {
	return obj{"type": "message", "timestamp": "2026-09-01T00:00:00Z", "message": obj{"role": "user", "content": []obj{{"type": "text", "text": text}}}}
}

func claudeDir(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-", "_", "-", "~", "-", " ", "-").Replace(cwd)
}

// claudeSession writes a main Claude session. Each user line carries the cwd
// and sessionId, as in real files.
func (f *fixture) claudeSession(cwd, id string, hour int, lines ...any) string {
	path := filepath.Join(f.home, ".claude", "projects", claudeDir(cwd), id+".jsonl")
	head := []any{
		obj{"type": "mode", "mode": "normal", "sessionId": id},
		obj{"type": "permission-mode", "permissionMode": "auto", "sessionId": id},
	}
	return f.write(path, hour, append(head, stamp(lines, cwd, id)...)...)
}

func (f *fixture) claudeSubagent(cwd, parentID, agentID string, hour int, meta obj, lines ...any) string {
	dir := filepath.Join(f.home, ".claude", "projects", claudeDir(cwd), parentID, "subagents")
	metaPath := filepath.Join(dir, "agent-"+agentID+".meta.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	stamped := stamp(lines, cwd, parentID)
	for _, l := range stamped {
		if m, ok := l.(obj); ok {
			m["isSidechain"] = true
			m["agentId"] = agentID
		}
	}
	return f.write(filepath.Join(dir, "agent-"+agentID+".jsonl"), hour, stamped...)
}

func stamp(lines []any, cwd, id string) []any {
	for _, l := range lines {
		if m, ok := l.(obj); ok && (m["type"] == "user" || m["type"] == "assistant") {
			m["cwd"] = cwd
			m["sessionId"] = id
			if _, ok := m["entrypoint"]; !ok {
				m["entrypoint"] = "cli"
			}
		}
	}
	return lines
}

func claudeUser(content any) obj {
	return obj{"type": "user", "isSidechain": false, "userType": "external", "message": obj{"role": "user", "content": content}}
}

func claudeAssistant(text string) obj {
	return obj{"type": "assistant", "message": obj{"role": "assistant", "content": []obj{{"type": "text", "text": text}}}}
}

// codexSession writes a date-organised rollout file. meta overrides fields of
// the session_meta payload.
func (f *fixture) codexSession(cwd, id string, hour int, meta obj, userMessages ...string) string {
	payload := obj{
		"id": id, "session_id": id, "cwd": cwd, "originator": "codex-tui", "source": "cli",
		"thread_source": "user", "cli_version": "0.149.0",
		// real files embed the full system prompt here
		"base_instructions": obj{"text": strings.Repeat("You are Codex. ", 3000)},
	}
	for k, v := range meta {
		payload[k] = v
	}
	lines := []any{obj{"timestamp": "2026-09-01T00:00:00Z", "type": "session_meta", "payload": payload}}
	for _, m := range userMessages {
		lines = append(lines,
			obj{"type": "response_item", "payload": obj{"type": "message", "role": "user", "content": []obj{{"type": "input_text", "text": "<environment_context>...</environment_context>"}}}},
			obj{"type": "event_msg", "payload": obj{"type": "user_message", "message": m, "images": []any{}}},
			obj{"type": "event_msg", "payload": obj{"type": "agent_message", "message": "ok"}},
		)
	}
	path := filepath.Join(f.home, ".codex", "sessions", "2026", "09", "01", "rollout-2026-09-01T12-00-00-"+id+".jsonl")
	return f.write(path, hour, lines...)
}

func (f *fixture) codexIndex(entries ...obj) {
	path := filepath.Join(f.home, ".codex", "session_index.jsonl")
	lines := make([]any, len(entries))
	for i, e := range entries {
		lines[i] = e
	}
	f.write(path, 0, lines...)
}

func piDir(cwd string) string {
	return "--" + strings.TrimPrefix(strings.ReplaceAll(cwd, "/", "-"), "-") + "--"
}

// piSession writes a pi session and returns its path. start overrides fields
// of the first "session" line (e.g. parentSession).
func (f *fixture) piSession(cwd, id string, hour int, start obj, lines ...any) string {
	head := obj{"type": "session", "version": 3, "id": id, "timestamp": "2026-09-01T00:00:00Z", "cwd": cwd}
	for k, v := range start {
		head[k] = v
	}
	path := filepath.Join(f.home, ".pi", "agent", "sessions", piDir(cwd), "2026-09-01T00-00-00-000Z_"+id+".jsonl")
	return f.write(path, hour, append([]any{head}, lines...)...)
}

func piUser(text string) obj {
	return obj{"type": "message", "id": "m", "message": obj{"role": "user", "content": []obj{{"type": "text", "text": text}}}}
}

func piName(name string) obj {
	return obj{"type": "session_info", "id": "i", "name": name}
}

func droidAssistant(text string) obj {
	return obj{"type": "message", "message": obj{"role": "assistant", "content": []obj{{"type": "text", "text": text}}}}
}
