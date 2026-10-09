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
	b.WriteString(SubMarker(s.HiddenSubagents, false))
	return b.String()
}

// SubMarker labels a row with n sub-agents: "+n sub" while they are hidden,
// "−n sub" while they are listed below it, nothing when n is 0.
func SubMarker(n int, listed bool) string {
	switch {
	case n == 0:
		return ""
	case listed:
		return fmt.Sprintf("  −%d sub", n)
	}
	return fmt.Sprintf("  +%d sub", n)
}

// Row is one flattened tree entry with its tree-drawing prefix.
type Row struct {
	Session *agent.Session
	Parent  *agent.Session // nil for top-level rows
	Prefix  string
}

// Flatten walks the forest depth-first, producing "├─ " / "└─ " prefixes. It
// descends into a session's children only when open reports true for it; a
// nil open descends everywhere.
func Flatten(ss []*agent.Session, open func(*agent.Session) bool) []Row {
	var rows []Row
	var walk func(ss []*agent.Session, parent *agent.Session, indent string)
	walk = func(ss []*agent.Session, parent *agent.Session, indent string) {
		for i, s := range ss {
			prefix, next := "", ""
			if parent != nil {
				if i == len(ss)-1 {
					prefix, next = indent+"└─ ", indent+"   "
				} else {
					prefix, next = indent+"├─ ", indent+"│  "
				}
			}
			rows = append(rows, Row{Session: s, Parent: parent, Prefix: prefix})
			if open == nil || open(s) {
				walk(s.Children, s, next)
			}
		}
	}
	walk(ss, nil, "")
	return rows
}
