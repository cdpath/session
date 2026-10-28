package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/display"
)

func TestVersionFlag(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		var out, errOut bytes.Buffer
		if code := run([]string{flag}, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d, stderr %q", flag, code, errOut.String())
		}
		if !strings.HasPrefix(out.String(), "session ") || strings.Count(out.String(), "\n") != 1 {
			t.Errorf("%s: output %q, want one line starting with \"session \"", flag, out.String())
		}
	}
}

func TestGlobalTableIncludesRepoColumn(t *testing.T) {
	sessions := []*agent.Session{
		{Agent: "droid", ID: "d1", Title: "alpha", Turns: 1, Cwd: "/home/u/repo",
			RepoSlug: "org/repo", Updated: time.Now()},
		{Agent: "claude", ID: "c1", Title: "bravo", Turns: 2, Cwd: "/home/u/other",
			RepoDirName: "other", Updated: time.Now()},
	}
	var out bytes.Buffer
	printTable(&out, sessions, true, "/home/u")
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if !strings.Contains(lines[0], "REPO") {
		t.Errorf("header missing REPO column: %q", lines[0])
	}
	if !strings.Contains(lines[1], "org/repo") {
		t.Errorf("slug missing from row: %q", lines[1])
	}
	if !strings.Contains(lines[2], "other") {
		t.Errorf("dir-name fallback missing from row: %q", lines[2])
	}
}

func TestGlobalTableOmitsRepoColumnWhenNotGlobal(t *testing.T) {
	sessions := []*agent.Session{
		{Agent: "droid", ID: "d1", Title: "alpha", Turns: 1, Cwd: "/repo",
			RepoSlug: "org/repo", Updated: time.Now()},
	}
	var out bytes.Buffer
	printTable(&out, sessions, false, "/home/u")
	if strings.Contains(out.String(), "REPO") {
		t.Errorf("non-global table should not have REPO column: %s", out.String())
	}
}

func TestJSONRepoField(t *testing.T) {
	s := agent.Session{Agent: "droid", ID: "d1", RepoSlug: "org/repo"}
	data, _ := json.Marshal(s)
	if !strings.Contains(string(data), `"repo":"org/repo"`) {
		t.Errorf("JSON should contain repo field: %s", data)
	}

	s2 := agent.Session{Agent: "droid", ID: "d2"}
	data2, _ := json.Marshal(s2)
	if strings.Contains(string(data2), `"repo"`) {
		t.Errorf("JSON should omit empty repo: %s", data2)
	}

	s3 := agent.Session{Agent: "droid", ID: "d3", RepoDirName: "local"}
	data3, _ := json.Marshal(s3)
	if strings.Contains(string(data3), "local") || strings.Contains(string(data3), "repoDirName") {
		t.Errorf("JSON should never contain RepoDirName: %s", data3)
	}
}

func TestRepoLabelUsedInTable(t *testing.T) {
	s := &agent.Session{Agent: "a", ID: "1", Turns: 1, Updated: time.Now(), RepoDirName: "proj"}
	got := display.RepoLabel(s)
	if got != "proj" {
		t.Errorf("RepoLabel fallback = %q, want proj", got)
	}
}
