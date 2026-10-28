# session

List your AI coding-agent sessions ([pi](https://github.com/badlogic/pi-mono), Claude Code, Codex, Factory Droid) in one picker and resume the selected one with the right agent.

```sh
go install github.com/cdpath/session/cmd/session@latest
```

## Usage

```sh
session                 # sessions started in the current directory
session -r              # ...and its subdirectories
session -g              # sessions from every directory
session -a claude,codex # only some agents
session --subagents     # sub-agent sessions as a tree under their parent (picker: start expanded)
session --headless      # include SDK / integration sessions
session --json          # machine-readable output (includes repo slug)
session --refresh       # recompute cached repo slugs after changing a remote
session -- --model x    # append arguments to the resume command
session -v              # print the version (include it in bug reports)
```

Releases are tagged `vX.Y.Z` and `go install …@latest` tracks the newest tag. To install the newest commit from `main` instead:

```sh
GOPROXY=direct go install github.com/cdpath/session/cmd/session@main
```

In the picker:

| key | action |
| --- | --- |
| `↑/↓`, `j/k` | move |
| `PgDn`/`d`/`ctrl+f`, `PgUp`/`u`/`b`/`ctrl+b` | next / previous page |
| `→`/`l`, `←`/`h` | expand / collapse a session's sub-agents (rows marked `+N sub`) |
| `f` | only sessions with sub-agents, all expanded, plus orphaned sub-agents |
| `/` | filter |
| `Tab` | switch agent |
| `s` | switch scope: current directory → with subdirectories → all directories |
| `Enter` | resume |
| `q` | quit |

`-r` and `-g` choose the scope the picker starts in, and `--subagents` starts it with every tree expanded and orphaned sub-agents shown.

On Enter, `session` changes into the session's original directory and replaces itself with the agent:

| agent  | resume command                 |
| ------ | ------------------------------ |
| pi     | `pi --session <session file>`  |
| claude | `claude --resume <id>`         |
| codex  | `codex resume <id>`            |
| droid  | `droid --resume <id>`          |

Claude sub-agents can't be resumed on their own, so selecting one resumes its parent session.

Each row shows the **repo slug** (`owner/repo`, parsed from the git remote) so you can tell which project a session belongs to. Repos on hosts other than `github.com` include the host prefix; repos without a remote show the directory name as a faint fallback.

When stdout is not a terminal, `session` prints a plain table instead of the picker. The global table (`-g`) includes a `REPO` column, and `--json` adds a `repo` field (slug only, omitted when empty).

Session metadata is cached in your user cache directory (`~/Library/Caches/session` on macOS, `~/.cache/session` on Linux). Use `--no-cache` or `--rebuild-cache` if something looks stale, `--refresh` to recompute repo slugs after changing a remote, and `--debug` to see files that could not be parsed.

Developers: `make build` / `make test` build and test; `make tag VERSION=vX.Y.Z` tests, tags and pushes a release.

`session` reads each agent's session files directly. Those formats are undocumented and may change; unreadable files are skipped rather than breaking the list. macOS and Linux only.

See [docs/prd](docs/prd) for the full design.
