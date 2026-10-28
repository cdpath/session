package reposlug

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSlug(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo", "owner/repo"},
		{"git@github.com:owner/repo.git", "owner/repo"},
		{"git@github.com:owner/repo", "owner/repo"},
		{"ssh://git@github.com/owner/repo.git", "owner/repo"},
		{"git://github.com/owner/repo.git", "owner/repo"},
		{"https://user:pass@github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo/", "owner/repo"},

		{"https://gitlab.com/owner/repo.git", "gitlab.com/owner/repo"},
		{"git@gitlab.com:owner/repo.git", "gitlab.com/owner/repo"},
		{"https://github.example.com/owner/repo", "github.example.com/owner/repo"},

		{"", ""},
		{"/local/path/to/repo.git", ""},
		{"file:///local/repo.git", ""},
		{"https://github.com/owner", ""},
		{"https://github.com/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := ParseSlug(tt.url); got != tt.want {
				t.Errorf("ParseSlug(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestForDir_RepoWithOrigin(t *testing.T) {
	dir := t.TempDir()
	writeGitConfig(t, filepath.Join(dir, ".git"), `[remote "origin"]
	url = https://github.com/acme/app.git
`)
	r := ForDir(dir)
	if r.Slug != "acme/app" {
		t.Errorf("slug = %q, want acme/app", r.Slug)
	}
	if r.Root != dir {
		t.Errorf("root = %q, want %q", r.Root, dir)
	}
}

func TestForDir_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	writeGitConfig(t, filepath.Join(dir, ".git"), `[remote "origin"]
	url = https://github.com/acme/app.git
`)
	sub := filepath.Join(dir, "pkg", "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	r := ForDir(sub)
	if r.Slug != "acme/app" {
		t.Errorf("slug = %q, want acme/app", r.Slug)
	}
}

func TestForDir_NoOriginFallsBackToFirstRemote(t *testing.T) {
	dir := t.TempDir()
	writeGitConfig(t, filepath.Join(dir, ".git"), `[remote "upstream"]
	url = https://github.com/upstream/lib.git
`)
	r := ForDir(dir)
	if r.Slug != "upstream/lib" {
		t.Errorf("slug = %q, want upstream/lib", r.Slug)
	}
}

func TestForDir_NoRemote(t *testing.T) {
	dir := t.TempDir()
	writeGitConfig(t, filepath.Join(dir, ".git"), "[core]\n\tbare = false\n")
	r := ForDir(dir)
	if r.Slug != "" {
		t.Errorf("slug = %q, want empty", r.Slug)
	}
	if got := r.DirName(); got == "" {
		t.Error("DirName() should be non-empty for a repo without a remote")
	}
}

func TestForDir_OutsideRepo(t *testing.T) {
	dir := t.TempDir()
	r := ForDir(dir)
	if r.Slug != "" || r.Root != "" {
		t.Errorf("got slug=%q root=%q, want empty", r.Slug, r.Root)
	}
	if got := r.DirName(); got != "" {
		t.Errorf("DirName() = %q, want empty", got)
	}
}

func TestForDir_Worktree(t *testing.T) {
	dir := t.TempDir()
	mainGit := filepath.Join(dir, "main", ".git")
	writeGitConfig(t, mainGit, `[remote "origin"]
	url = https://github.com/acme/wt.git
`)
	wtGitDir := filepath.Join(mainGit, "worktrees", "feature")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(dir, "worktree")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: "+wtGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := ForDir(wtDir)
	if r.Slug != "acme/wt" {
		t.Errorf("slug = %q, want acme/wt", r.Slug)
	}
	if r.Root != wtDir {
		t.Errorf("root = %q, want %q", r.Root, wtDir)
	}
}

func TestForDir_DeletedDirectory(t *testing.T) {
	r := ForDir(filepath.Join(t.TempDir(), "gone"))
	if r.Slug != "" || r.Root != "" {
		t.Errorf("got slug=%q root=%q, want empty", r.Slug, r.Root)
	}
}

func writeGitConfig(t *testing.T, gitDir, config string) {
	t.Helper()
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
}
