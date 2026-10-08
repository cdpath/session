// Package shared holds file-reading helpers common to all providers.
package shared

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// ProjectDirs returns the per-project subdirectories of root. Their names
// encode the cwd lossily, so the index matches on the cwd inside each file.
func ProjectDirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	return out, nil
}

// ProjectSessions lists *.jsonl files in the project directories of root;
// the layout pi and Droid share.
func ProjectSessions(root string) ([]string, error) {
	dirs, err := ProjectDirs(root)
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
