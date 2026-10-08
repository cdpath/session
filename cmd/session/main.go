// Command session lists AI coding-agent sessions (pi, Claude Code, Codex,
// Droid) and resumes the selected one with the right agent.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/cdpath/session/internal/agent"
	"github.com/cdpath/session/internal/display"
	"github.com/cdpath/session/internal/index"
	"github.com/cdpath/session/internal/resume"
	"github.com/cdpath/session/internal/tui"
)

const usage = `Usage: session [list] [flags] [-- agent-args...]

List agent sessions for the current directory and resume one.

Flags:
  -g, --global         list sessions from every directory
  -r, --recursive      include sessions from subdirectories
  -a, --agent LIST     only these agents (comma-separated: %s)
      --subagents      show sub-agent sessions as a tree under their parent
      --headless       include sessions started by SDKs and integrations
      --json           print JSON
      --no-cache       do not read or write the metadata cache
      --rebuild-cache  re-parse every session file and rewrite the cache
      --debug          report files that could not be parsed
  -v, --version        print the version and exit

Arguments after -- are appended to the agent's resume command.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type flags struct {
	global, recursive, subagents, headless, json bool
	noCache, rebuildCache, debug, version        bool
	agents                                       string
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "list" {
		args = args[1:]
	}
	var extra []string
	if i := slices.Index(args, "--"); i >= 0 {
		args, extra = args[:i], args[i+1:]
	}

	names := providerNames()
	var f flags
	fs := flag.NewFlagSet("session", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, usage, strings.Join(names, ",")) }
	for _, n := range []string{"g", "global"} {
		fs.BoolVar(&f.global, n, false, "")
	}
	for _, n := range []string{"r", "recursive"} {
		fs.BoolVar(&f.recursive, n, false, "")
	}
	for _, n := range []string{"a", "agent"} {
		fs.StringVar(&f.agents, n, "", "")
	}
	fs.BoolVar(&f.subagents, "subagents", false, "")
	fs.BoolVar(&f.headless, "headless", false, "")
	fs.BoolVar(&f.json, "json", false, "")
	fs.BoolVar(&f.noCache, "no-cache", false, "")
	fs.BoolVar(&f.rebuildCache, "rebuild-cache", false, "")
	fs.BoolVar(&f.debug, "debug", false, "")
	for _, n := range []string{"v", "version"} {
		fs.BoolVar(&f.version, n, false, "")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.version {
		fmt.Fprintln(stdout, versionString())
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "session: unexpected argument %q (pass agent arguments after --)\n", fs.Arg(0))
		return 2
	}

	var agents []string
	if f.agents != "" {
		for _, a := range strings.Split(f.agents, ",") {
			a = strings.TrimSpace(a)
			if !slices.Contains(names, a) {
				fmt.Fprintf(stderr, "session: unknown agent %q (known: %s)\n", a, strings.Join(names, ", "))
				return 2
			}
			agents = append(agents, a)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "session:", err)
		return 1
	}
	env := agent.OSEnv()
	res, err := index.List(index.Options{
		Env:          env,
		Cwd:          cwd,
		Global:       f.global,
		Recursive:    f.recursive,
		Subagents:    f.subagents,
		Headless:     f.headless,
		Agents:       agents,
		LookPath:     exec.LookPath,
		CachePath:    index.DefaultCachePath(),
		NoCache:      f.noCache,
		RebuildCache: f.rebuildCache,
	})
	if err != nil {
		fmt.Fprintln(stderr, "session:", err)
		return 1
	}
	if f.debug {
		for _, s := range res.Skipped {
			fmt.Fprintf(stderr, "skipped %s: %v\n", s.Path, s.Err)
		}
	}

	if f.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		sessions := res.Sessions
		if sessions == nil {
			sessions = []*agent.Session{}
		}
		if err := enc.Encode(sessions); err != nil {
			fmt.Fprintln(stderr, "session:", err)
			return 1
		}
		return 0
	}

	if len(res.Sessions) == 0 {
		where := "in " + display.ShortPath(cwd, env.Home)
		if f.global {
			where = "anywhere"
		}
		hint := ""
		if !f.global {
			hint = " (try -r or -g)"
		}
		fmt.Fprintf(stderr, "No sessions found %s%s.\n", where, hint)
		return 0
	}

	if !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		printTable(stdout, res.Sessions, f.global, env.Home)
		return 0
	}

	heading := display.ShortPath(cwd, env.Home)
	if f.global {
		heading = "all directories"
	}
	choice, err := tui.Run(tui.Config{
		Sessions: res.Sessions,
		Global:   f.global,
		Home:     env.Home,
		Heading:  heading,
		Agents:   presentAgents(res.Sessions, names),
	})
	if err != nil {
		fmt.Fprintln(stderr, "session:", err)
		return 1
	}
	if choice == nil {
		return 0
	}
	if choice.Agent == "claude" && choice.Subagent {
		fmt.Fprintf(stderr, "Claude sub-agents can't be resumed on their own; resuming parent session %s.\n", choice.ParentID)
	}
	fmt.Fprintf(stderr, "→ cd %s && %s\n", display.ShortPath(choice.Resume.Dir, env.Home), resume.String(*choice.Resume, extra))
	if err := resume.Exec(*choice.Resume, extra); err != nil {
		fmt.Fprintln(stderr, "session:", err)
		return 1
	}
	return 0
}

func providerNames() []string {
	var names []string
	for _, p := range index.Providers() {
		names = append(names, p.Name())
	}
	return names
}

// presentAgents keeps the Tab cycle to agents that actually have sessions.
func presentAgents(sessions []*agent.Session, order []string) []string {
	seen := map[string]bool{}
	for _, s := range sessions {
		seen[s.Agent] = true
	}
	var out []string
	for _, n := range order {
		if seen[n] {
			out = append(out, n)
		}
	}
	return out
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// printTable aligns columns by display width: titles are often CJK, which
// text/tabwriter would misalign.
func printTable(w io.Writer, sessions []*agent.Session, global bool, home string) {
	header := []string{"AGENT", "UPDATED", "TURNS", "TITLE"}
	if global {
		header = append(header, "CWD")
	}
	header = append(header, "ID")
	rows := [][]string{header}
	now := time.Now()
	for _, r := range display.Flatten(sessions) {
		s := r.Session
		title := r.Prefix + ansi.Truncate(display.Title(s), 70, "…")
		row := []string{s.Agent, display.Ago(s.Updated, now), fmt.Sprint(s.Turns), title}
		if global {
			row = append(row, display.ShortPath(s.Cwd, home))
		}
		row = append(row, s.ID)
		if s.Problem != "" {
			// trailing, unaligned column so it doesn't widen the others
			row = append(row, "cannot resume: "+s.Problem)
		}
		rows = append(rows, row)
	}
	widths := make([]int, len(header))
	for _, row := range rows {
		for i := range header {
			widths[i] = max(widths[i], ansi.StringWidth(row[i]))
		}
	}
	for _, row := range rows {
		var b strings.Builder
		for i, c := range row {
			b.WriteString(c)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-ansi.StringWidth(c)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
