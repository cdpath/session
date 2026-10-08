package index_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/index"
)

func session(id, cwd string, children ...*agent.Session) *agent.Session {
	return &agent.Session{Agent: "droid", ID: id, Title: id, Turns: 1, Cwd: cwd, Children: children}
}

func TestScopeFilter(t *testing.T) {
	forest := []*agent.Session{
		session("root", "/repo", session("child", "/elsewhere")),
		session("sub", "/repo/web/"),
		session("sibling", "/repo-old"),
		session("other", "/other"),
	}
	forest[3].HiddenSubagents = 2
	sc := index.NewScoper("/repo")

	assertList(t, sc.Filter(forest, index.Here), `
		droid root "root" turns=1
		  droid child "child" turns=1
	`)
	assertList(t, sc.Filter(forest, index.Subdirs), `
		droid root "root" turns=1
		  droid child "child" turns=1
		droid sub "sub" turns=1
	`)
	assertList(t, sc.Filter(forest, index.All), `
		droid root "root" turns=1
		  droid child "child" turns=1
		droid sub "sub" turns=1
		droid sibling "sibling" turns=1
		droid other "other" turns=1 +2 sub
	`)
}

func TestScopeFilterFromSymlinkedDirectory(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(home, "real")
	if err := os.MkdirAll(filepath.Join(real, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	forest := []*agent.Session{
		session("via-real", filepath.Join(real, "web")),
		session("via-link", link),
	}

	sc := index.NewScoper(link)
	assertList(t, sc.Filter(forest, index.Subdirs), `
		droid via-real "via-real" turns=1
		droid via-link "via-link" turns=1
	`)
	for p, want := range map[string]string{filepath.Join(real, "web"): "./web", link: "."} {
		if got, ok := sc.Rel(p); !ok || got != want {
			t.Errorf("Rel(%s) = %q, %v, want %q", p, got, ok, want)
		}
	}
	if got, ok := sc.Rel(home); ok {
		t.Errorf("Rel(parent) = %q, want not ok", got)
	}
}

func TestScopeCycle(t *testing.T) {
	s := index.Here
	var got []index.Scope
	for range 4 {
		s = s.Next()
		got = append(got, s)
	}
	want := []index.Scope{index.Subdirs, index.All, index.Here, index.Subdirs}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cycle = %v, want %v", got, want)
		}
	}
}
