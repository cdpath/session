// Package shared holds file-reading helpers common to all providers.
package shared

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/cdpath/session/internal/agent"
)

// ErrStop ends EachLine early without reporting an error.
var ErrStop = errors.New("stop")

// EachLine calls fn for every non-empty line of a JSONL file. Lines can be
// arbitrarily long (Codex embeds a ~20 KB system prompt in its first line).
func EachLine(path string, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if trimmed := trimEOL(line); len(trimmed) > 0 {
				if ferr := fn(trimmed); ferr != nil {
					if errors.Is(ferr, ErrStop) {
						return nil
					}
					return ferr
				}
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func trimEOL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// comparableName reduces a path or an agent's encoded directory name to a
// form where the two can be compared. Agents encode cwd into directory names
// lossily and inconsistently ('/', '.', '_', '~' may all become '-'), so the
// encoded name can't be decoded; instead both sides are squashed the same way.
func comparableName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > unicode.MaxASCII || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// ProjectDirs returns subdirectories of root whose encoded name could
// correspond to the scope. In global scope it returns all of them.
func ProjectDirs(root string, scope agent.Scope) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var targets []string
	for _, d := range scope.Dirs {
		targets = append(targets, comparableName(d))
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if scope.Global || matchesAny(comparableName(e.Name()), targets, scope.Recursive) {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	return out, nil
}

func matchesAny(name string, targets []string, recursive bool) bool {
	for _, t := range targets {
		if name == t {
			return true
		}
		if recursive && (t == "" || strings.HasPrefix(name, t+"-")) {
			return true
		}
	}
	return false
}

// ProjectSessions lists *.jsonl files in the project directories of root that
// may belong to the scope; the layout pi and Droid share.
func ProjectSessions(root string, scope agent.Scope) ([]string, error) {
	dirs, err := ProjectDirs(root, scope)
	if err != nil {
		return nil, err
	}
	return GlobJSONL(dirs), nil
}

// GlobJSONL returns *.jsonl files directly inside each dir.
func GlobJSONL(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		m, _ := filepath.Glob(filepath.Join(d, "*.jsonl"))
		out = append(out, m...)
	}
	return out
}

// OneLine collapses all whitespace runs into single spaces.
func OneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// IsInjected reports whether a user-role text was injected by the agent
// harness (system reminders, command echoes, notifications) rather than typed.
func IsInjected(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "<")
}

// FileStem returns the base name without extension.
func FileStem(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
