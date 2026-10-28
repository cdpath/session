# PRD: show the repo slug for each session

## Problem Statement

In the all-directories scope the list spans many projects, and the home-relative path is a weak identity: `~/Developer/session` tells me where a session ran, but not which project it belonged to. Paths are long and similar to each other; the thing I actually scan for — "the session in the session repo", "the one in pi-mono" — is the GitHub identity, which today the tool never shows.

## Solution

Each session gains a **repo slug**: the `owner/repo` identity of the git repository containing the session's working directory, parsed from the repository's remote URL. `github.com` remotes show as `owner/repo`; other hosts keep the host prefix (`gitlab.com/owner/repo`). A repository with no usable remote shows its root directory name instead (the **repo directory name** fallback); a session outside any repository shows nothing.

In the picker the slug sits before the working directory in a distinct low-saturation blue, so it reads as project identity rather than path. The global table gains a `REPO` column, and `--json` gains a `repo` field (slug only). Slugs are computed once per session file and cached with its parse result; `--refresh` recomputes cached slugs after a remote changes, without re-parsing session files.

## User Stories

### Identifying the project

1. As a developer, I want each picker row (in the subdirs and all-directories scopes) to show the repo slug next to its working directory, so that I can tell at a glance which project a session belongs to.
2. As a developer, I want the slug to come from the git repository containing the session's working directory — however deep inside it the session started, and in linked worktrees too — so that the identity is right regardless of where I launched the agent.
3. As a developer, I want remote URLs in any common form normalized (https, `git@github.com:owner/repo.git` SCP-style, `git://`, `.git` suffix, embedded credentials), so that the slug is always clean `owner/repo`.
4. As a developer, I want non-`github.com` remotes shown with their host (`gitlab.com/owner/repo`), so that GitLab and GitHub Enterprise repos are identifiable too.
5. As a developer, I want a repository with no usable remote to show its root directory name in the same faint style as the path, so that unpushed local repos still have a project label — visually distinct from a real slug, which is colored.
6. As a developer, I want a session outside any git repository to show nothing, so that the slot is never noise.
7. As a developer, I want the slug styled in a distinct low-saturation blue while paths stay faint, so that identity and location read as different kinds of information.

### Fitting the existing list

8. As a developer, I want no slug in the current-directory scope (it would be the same on every row), so that the row stays as short as today.
9. As a developer, I want a sub-agent row to hide the slug when it matches its parent's (the cwd rule), so that tree rows only call out differences.
10. As a developer, I want the slug dropped before the path when the terminal is narrow, so that title and recency survive width pressure.
11. As a developer, I want `/` to match the displayed repo label (slug or directory name), so that I can jump to a project by typing its name.
12. As a developer, I want the current-directory picker to behave exactly as before, so that nothing changes for the default case.

### Other outputs

13. As a developer, I want the global table to gain a `REPO` column between `TITLE` and `CWD`, so that scripted and non-interactive output carries the same identity.
14. As a developer, I want `--json` to gain a `repo` field containing the slug only (omitted when empty, never the directory-name fallback), so that consumers can rely on `owner/repo` being splittable.

### Cache and freshness

15. As a developer, I want slugs computed once per session file and stored in the metadata cache, so that warm runs do no extra filesystem work.
16. As a developer, I want `--refresh` to recompute every cached slug (walking each unique working directory once) without re-parsing session files, so that changing a remote is fixed in milliseconds instead of a full rebuild.

## Implementation Decisions

### Slug derivation

- From the session's `Cwd`, walk up to the nearest `.git`: a directory, or a file in a linked worktree — in that case resolve `gitdir:` and read the shared repository config (for worktrees, the config two levels above the resolved git dir; submodules resolve to `.git/modules/<name>` and report their own remote, which is the correct identity).
- Read `config` directly; never spawn `git`.
- Remote choice: `origin` first; otherwise the first remote in config order; otherwise no usable remote.
- URL parsing: strip credentials and `.git` suffix; recognize `https://host/owner/repo`, `git@host:owner/repo`, `ssh://git@host/owner/repo`, `git://host/owner/repo`. A usable remote yields `owner/repo` for `github.com`, `host/owner/repo` for other hosts. Anything else (local paths, bare URLs without an owner) counts as no usable remote.
- A missing directory (session's `Cwd` deleted) is simply not a repository: blank label.

### Session record and cache

- `Session` gains `RepoSlug string` **above** the derived marker (cacheable), JSON tag `repo,omitempty`. The directory-name fallback is display-layer only, derived at render time from the repository root.
- Slugs are computed centrally in the index after parsing (not per provider), memoized per unique `Cwd` per run, then stored in the cache entry with the parse result.
- `schemaVersion` bumps 1→2: existing caches are discarded and rebuilt once on the first run after upgrading.
- A remote changing later does not refresh cached slugs; `--refresh` is the documented escape hatch.

### `--refresh`

- Loads the cache (no-op with `--no-cache` or `--rebuild-cache`), recomputes `RepoSlug` for every stored session — deduplicated by `Cwd` — writes the cache back, then proceeds with a normal listing. Session files are stat'ed but never re-parsed.

### Picker

- Row layout: `<tree> <agent> <ago> <turns> <title> <slug> <cwd>`, slug colored `75`, cwd faint as today; the directory-name fallback is rendered in the cwd slot's faint style (uncolored), keeping it visibly distinct from a real slug.
- Visibility follows the cwd rules exactly: hidden in the current-directory scope, hidden on sub-agent rows whose parent shows the same label, and under width pressure dropped before the cwd.
- `FilterValue` gains the displayed repo label.

### Table and JSON

- Global table header becomes `AGENT UPDATED TURNS TITLE REPO CWD ID` (mirroring the picker's slug-before-path order), showing slug or directory-name fallback.
- JSON `repo` field: slug only.

### Documentation

- README gains `--refresh` in the flag list ("recompute cached repo slugs, e.g. after changing a remote") and the row description mentions the repo slug. `GLOSSARY.md` defines repo slug and repo directory name.

## Testing Decisions

- URL normalization: table-driven test over the URL forms above (plus `.git` suffix, credentials, non-github hosts, local-path remotes → no usable remote).
- Repository discovery: session in a repo subdirectory, in a linked worktree, with `origin`, without `origin` but another remote, with no remote (directory-name fallback), and outside any repository (blank).
- Cache: slug persists across runs; a v1 cache is discarded and rebuilt; `--refresh` rewrites slugs after a fixture remote changes, without touching parse results.
- Rendering: picker row (slug colored, fallback faint, hidden in the current-directory scope, dropped before cwd when narrow) and global table column, following the existing TUI test patterns.

## Out of Scope

- Making the slug clickable or opening the repository.
- Refreshing slugs automatically when a remote changes.
- Suppressing repeated slugs across consecutive rows (grouping).
- Special-casing submodules beyond what natural `.git` discovery gives.
