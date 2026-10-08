// Package display formats sessions for the table and the TUI.
package display

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cdpath/session/internal/agent"
)

// Ago formats t relative to now, e.g. "3h ago"; older times show the date.
func Ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return t.Local().Format("2006-01-02")
	}
}

// ShortPath replaces the home directory prefix with "~".
func ShortPath(p, home string) string {
	if home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rel, ok := strings.CutPrefix(p, home+string(filepath.Separator)); ok {
		return "~/" + rel
	}
	return p
}

// Title is the row label: optional [tag], title, and hidden sub-agent hint.
func Title(s *agent.Session) string {
	var b strings.Builder
	if s.Tag != "" {
		fmt.Fprintf(&b, "[%s] ", s.Tag)
	}
	t := s.DisplayTitle()
	if t == "" {
		t = "(untitled)"
	}
	b.WriteString(t)
	if s.Orphan {
		b.WriteString(" (orphan)")
	}
	if s.HiddenSubagents > 0 {
		fmt.Fprintf(&b, "  +%d sub", s.HiddenSubagents)
	}
	return b.String()
}

// Row is one flattened tree entry with its tree-drawing prefix.
type Row struct {
	Session *agent.Session
	Prefix  string
}

// Flatten walks the forest depth-first, producing "├─ " / "└─ " prefixes.
func Flatten(ss []*agent.Session) []Row {
	var rows []Row
	var walk func(ss []*agent.Session, indent string, child bool)
	walk = func(ss []*agent.Session, indent string, child bool) {
		for i, s := range ss {
			last := i == len(ss)-1
			prefix, next := "", ""
			if child {
				if last {
					prefix, next = indent+"└─ ", indent+"   "
				} else {
					prefix, next = indent+"├─ ", indent+"│  "
				}
			}
			rows = append(rows, Row{Session: s, Prefix: prefix})
			walk(s.Children, next, true)
		}
	}
	walk(ss, "", false)
	return rows
}
