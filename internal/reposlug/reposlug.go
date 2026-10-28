// Package reposlug resolves a git repository's remote identity from its
// working directory, producing an owner/repo slug for display.
package reposlug

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Result holds the repo slug and root directory for a working directory.
type Result struct {
	Slug string // owner/repo or host/owner/repo; empty if no usable remote
	Root string // absolute path to the repository root; empty if not in a repo
}

// DirName returns the fallback label: the base name of the repo root.
func (r Result) DirName() string {
	if r.Root == "" {
		return ""
	}
	return filepath.Base(r.Root)
}

// ForDir resolves the repo slug for a working directory by walking up to
// find .git, reading the git config, and parsing the remote URL.
func ForDir(dir string) Result {
	root, gitDir := findRepo(dir)
	if root == "" {
		return Result{}
	}
	configPath := filepath.Join(configDir(gitDir), "config")
	rawURL := readRemoteURL(configPath)
	return Result{Slug: ParseSlug(rawURL), Root: root}
}

// findRepo walks up from dir to find a .git directory or file.
func findRepo(dir string) (root, gitDir string) {
	dir = filepath.Clean(dir)
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Lstat(gitPath)
		if err == nil {
			if info.IsDir() {
				return dir, gitPath
			}
			if target := readGitFile(gitPath); target != "" {
				return dir, target
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

// readGitFile reads a .git file and resolves its gitdir: pointer.
func readGitFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(data))
	if !strings.HasPrefix(s, "gitdir: ") {
		return ""
	}
	target := s[len("gitdir: "):]
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target)
}

// configDir resolves the directory containing the git config file.
// For worktrees, the commondir file points to the shared .git directory.
// For normal repos and submodules (no commondir), it returns gitDir itself.
func configDir(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return gitDir
	}
	common := strings.TrimSpace(string(data))
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	return filepath.Clean(common)
}

// readRemoteURL reads the first usable remote URL from a git config file.
// Prefers "origin"; falls back to the first remote in file order.
func readRemoteURL(configPath string) string {
	f, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	var (
		inRemote bool
		name     string
		origin   string
		first    string
	)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			inRemote = false
			if strings.HasPrefix(line, `[remote "`) {
				name = strings.TrimSuffix(strings.TrimPrefix(line, `[remote "`), `"]`)
				inRemote = true
			}
			continue
		}
		if !inRemote {
			continue
		}
		if eq := strings.IndexByte(line, '='); eq >= 0 {
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			if key == "url" {
				if name == "origin" && origin == "" {
					origin = val
				}
				if first == "" {
					first = val
				}
			}
		}
	}
	if origin != "" {
		return origin
	}
	return first
}

// ParseSlug parses a git remote URL into a repository slug.
// Returns owner/repo for github.com, host/owner/repo for other hosts,
// or empty for unrecognized or local URLs.
func ParseSlug(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if strings.Contains(rawURL, "://") {
		u, err := url.Parse(rawURL)
		if err != nil || u.Host == "" || u.Scheme == "file" {
			return ""
		}
		return formatSlug(u.Hostname(), u.Path)
	}
	if colon := strings.IndexByte(rawURL, ':'); colon > 0 && !strings.HasPrefix(rawURL, "/") {
		host := rawURL[:colon]
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:]
		}
		return formatSlug(host, rawURL[colon+1:])
	}
	return ""
}

func formatSlug(host, path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	if host == "" || path == "" {
		return ""
	}
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	slug := parts[0] + "/" + parts[1]
	if host == "github.com" {
		return slug
	}
	return host + "/" + slug
}
