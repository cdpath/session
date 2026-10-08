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
session --subagents     # show sub-agent sessions as a tree under their parent
session --headless      # include SDK / integration sessions
session --json          # machine-readable output
session -- --model x    # append arguments to the resume command
```

In the picker: `↑/↓` or `j/k` to move, `/` to filter, `Tab` to switch agent, `Enter` to resume, `q` to quit.

On Enter, `session` changes into the session's original directory and replaces itself with the agent:

| agent  | resume command                 |
| ------ | ------------------------------ |
| pi     | `pi --session <session file>`  |
| claude | `claude --resume <id>`         |
| codex  | `codex resume <id>`            |
| droid  | `droid --resume <id>`          |

Claude sub-agents can't be resumed on their own, so selecting one resumes its parent session.

When stdout is not a terminal, `session` prints a plain table instead of the picker.

Session metadata is cached in your user cache directory (`~/Library/Caches/session` on macOS, `~/.cache/session` on Linux). Use `--no-cache` or `--rebuild-cache` if something looks stale, and `--debug` to see files that could not be parsed.

`session` reads each agent's session files directly. Those formats are undocumented and may change; unreadable files are skipped rather than breaking the list. macOS and Linux only.

See [docs/prd/0001-session-cli.md](docs/prd/0001-session-cli.md) for the full design.
