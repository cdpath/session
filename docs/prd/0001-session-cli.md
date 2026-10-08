# PRD: `session` — list and resume AI coding-agent sessions

## Problem Statement

I use several terminal coding agents (pi, Claude Code, Codex, Factory Droid) across many projects. Each one keeps its own session history in its own format and location, and each has its own resume picker that only knows about its own sessions. When I come back to a project I can't see, in one place, "what conversations did I have here, with which agent, and when" — and I can't jump back into one without remembering which agent it was and which flag that agent uses to resume. Across all projects it is worse: there is no global view at all, and sub-agent sessions (spawned by a main agent) are invisible or mixed in with real ones.

## Solution

A single Go CLI, `session`, that discovers sessions from all supported agents by reading their on-disk session files, and shows them in one interactive list.

- Run `session` (alias for `session list`) in a directory to see every session whose working directory is that directory, newest first, across all agents.
- Run `session list -g` to see sessions from every directory.
- Move through the list, fuzzy-filter it, switch agent filters, and press Enter: the tool changes into the session's original working directory and replaces itself with the right agent's resume command (`pi`, `claude`, `codex`, or `droid`), so the agent takes over the terminal as if launched directly.
- Sub-agent sessions are hidden by default; `--subagents` shows them as a tree beneath the session that spawned them.
- When stdout is not a terminal, it prints a plain table (or JSON with `--json`) so it composes with other tools.

## User Stories

### Listing in the current directory

1. As a developer, I want to run `session` in a project directory and see all agent sessions started in that directory, so that I can pick up where I left off without remembering which agent I used.
2. As a developer, I want `session` with no subcommand to behave exactly like `session list`, so that the common case is the shortest to type.
3. As a developer, I want sessions from pi, Claude Code, Codex, and Droid merged into one list, so that I don't have to open four separate pickers.
4. As a developer, I want the list sorted by last-updated time, newest first, so that the session I most likely want is at the top.
5. As a developer, I want each row to show the agent as a colored tag, so that I can tell at a glance which tool a session belongs to.
6. As a developer, I want each row to show a relative time such as "3h ago", so that I can judge recency quickly.
7. As a developer, I want each row to show a title, so that I can recognize what the session was about.
8. As a developer, I want the title to come from the agent's own title or name when one exists, so that I see the most meaningful label available.
9. As a developer, I want the title to fall back to the first real user message, truncated to one line, so that untitled sessions are still recognizable.
10. As a developer, I want each row to show the number of user turns, so that I can tell a quick question from a long working session.
11. As a developer, I want the default view to match only sessions whose working directory is exactly the current directory, so that the list matches how the agents themselves scope sessions.
12. As a developer, I want `-r` to also include sessions from subdirectories of the current directory, so that I can see sessions started anywhere inside a project.
13. As a developer, I want path matching to see through symlinks and path aliases (for example `/tmp` vs `/private/tmp`), so that sessions don't disappear because of how a path was spelled.

### Global listing

14. As a developer, I want `session list -g` to list sessions from every directory, so that I can find a conversation when I don't remember which project it was in.
15. As a developer, I want the global view to show a shortened working directory (for example `~/Developer/foo`) on each row, so that I know which project each session belongs to.
16. As a developer, I want the global view to stay fast even with roughly a thousand session files, so that it is usable as an everyday command.

### Filtering and navigation

17. As a developer, I want to move through the list with arrow keys and j/k, so that navigation feels like other terminal pickers.
18. As a developer, I want `/` to fuzzy-filter the list by title and other visible text, so that I can find a session by keyword.
19. As a developer, I want `-a claude,codex` to restrict the list to specific agents, so that I can narrow results from the command line.
20. As a developer, I want Tab in the TUI to cycle through All → pi → claude → codex → droid, so that I can change the agent filter without restarting.
21. As a developer, I want a clear way to quit the TUI without resuming anything, so that browsing is safe.

### Resuming

22. As a developer, I want Enter on a session to resume it with the correct agent, so that I don't need to remember each agent's resume syntax.
23. As a developer, I want the tool to change into the session's original working directory before resuming, so that agents that scope sessions by directory find the session and the agent works in the right project.
24. As a developer, I want the tool to replace its own process with the agent, so that the agent owns the terminal fully and no wrapper process lingers.
25. As a developer, I want pi sessions resumed by full session file path, so that partial-ID ambiguity can't pick the wrong session.
26. As a developer, I want to pass extra arguments after `--` (for example `session -- --dangerously-skip-permissions`), so that I can resume with the same flags I'd normally launch the agent with.
27. As a developer, I want sessions whose agent binary is not installed to still appear but dimmed, so that I can see my history even on a machine missing that agent.
28. As a developer, I want sessions whose working directory no longer exists to still appear but dimmed, so that I understand why they can't be resumed.
29. As a developer, I want Enter on a non-resumable session to show a clear error and keep me in the list, so that a mistake doesn't kick me out of the TUI.

### Sub-agent sessions

30. As a developer, I want sub-agent sessions hidden by default, so that the list shows only conversations I started myself.
31. As a developer, I want a parent row to show a hint like "+3 sub" when it has hidden sub-agents, so that I know more detail exists.
32. As a developer, I want `--subagents` to show sub-agent sessions indented beneath their parent session with tree connectors, so that I can see which main session spawned which sub-agent.
33. As a developer, I want the tree to nest recursively, so that sub-agents spawned by sub-agents are shown at the correct depth.
34. As a developer, I want siblings within a tree sorted by last-updated time, so that ordering is consistent with the top level.
35. As a developer, I want a sub-agent to appear under its parent regardless of the sub-agent's own working directory, so that the tree is never split across directory views.
36. As a developer, I want a sub-agent whose parent can't be found to appear at the top level marked "(orphan)", so that nothing silently disappears.
37. As a developer, I want Claude sub-agent rows titled with the sub-agent's task description and tagged with its agent type, so that I can tell sub-agents apart.
38. As a developer, I want Codex sub-agent rows tagged with their sub-agent kind (for example review or guardian), so that I know what role they played.
39. As a developer, I want Enter on a Claude sub-agent to resume its parent session with a one-line notice, because Claude sub-agents can't be resumed on their own.
40. As a developer, I want Enter on a Codex or Droid sub-agent to resume that sub-agent session itself, because those agents treat sub-agents as full sessions.

### Noise filtering

41. As a developer, I want sessions with no real user message (including hook-only sessions) always hidden, so that empty or aborted sessions don't clutter the list.
42. As a developer, I want headless and SDK-originated sessions (for example `claude -p`, `droid exec`, integrations like repoprompt or agent SDKs) hidden by default, so that automation runs don't drown out interactive work.
43. As a developer, I want `--headless` to include those sessions, so that I can still find them when I need to.
44. As a developer, I want Codex Desktop sessions treated as normal sessions, because the Codex CLI can resume them.

### Scripting and output

45. As a developer, I want a plain table when stdout is not a terminal, so that `session -g | grep foo` works.
46. As a developer, I want `--json` output with absolute timestamps, IDs, paths, agent, title, and parent links, so that I can script on top of the tool.

### Configuration, cache, and diagnostics

47. As a developer, I want each agent's own storage-location override respected (for example `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, pi's session-dir variables, Factory's home override), so that the tool finds sessions where the agent actually writes them.
48. As a developer, I want parsed session metadata cached and reused for unchanged files, so that repeated listings are fast.
49. As a developer, I want the cache automatically discarded when the tool's parsing logic changes, so that upgrades never show stale or wrong data.
50. As a developer, I want `--no-cache` and `--rebuild-cache`, so that I can bypass or reset the cache when debugging.
51. As a developer, I want unparseable or unexpected session files skipped rather than crashing the tool, so that one agent changing its format doesn't break the whole list.
52. As a developer, I want `--debug` to report which files were skipped and why, so that I can diagnose missing sessions.

### Installation

53. As a developer, I want to install with `go install`, so that setup is one command.
54. As a developer on macOS or Linux, I want the tool to just work there, so that it fits my daily environment.

## Implementation Decisions

### Command surface

- Binary name: `session`. Default subcommand: `list`. Room is left for future subcommands (e.g. `show`, `rm`) but none are in scope.
- Flags on `list`: `-g` (global), `-r` (include subdirectories; ignored with `-g`), `-a <agents>` (comma-separated agent filter), `--subagents`, `--headless`, `--json`, `--no-cache`, `--rebuild-cache`, `--debug`. Everything after `--` is passthrough arguments appended to the resume command.
- Interactive TUI when stdout is a TTY; plain table otherwise; `--json` forces JSON.

### Modules

- **Provider** (one per agent: pi, claude, codex, droid). A small, deep interface: given storage roots and a scope, discover session files and parse each into a common session record; build the resume command for a record. Providers own all agent-specific knowledge (paths, encodings, field names, sub-agent detection, title rules, headless detection, resume syntax). Adding a new agent means adding one provider.
- **Index**. Orchestrates providers concurrently (one goroutine per provider plus a bounded pool for file parsing), applies the cache, normalizes paths, applies scope/agent/subagent/headless filters, links sub-agents to parents, and returns a sorted forest of sessions. This is the single entry point the CLI and TUI consume.
- **Cache**. Persists parsed session records keyed by (file path, size, mtime) with a schema version; mismatched schema version discards the whole cache.
- **TUI**. bubbletea + bubbles list. Renders the forest, fuzzy filter on `/`, Tab cycles agent filter, Enter selects, quit without action. Renders non-resumable rows dimmed and shows errors inline without exiting.
- **Resume**. Given a selected record and passthrough args: verify the agent binary is on PATH and the working directory exists, chdir into it, then replace the process via exec. No shell wrapper; the parent shell's directory is unchanged after the agent exits.
- **CLI entry**. Flag parsing, TTY detection, output-mode selection, wiring Index → TUI/table/JSON → Resume.

### Common session record

Each provider produces records with: agent, session ID, source file path, working directory (from file contents), title, first user message, user-turn count, last-updated time, parent session ID (if sub-agent), sub-agent kind/tag (if any), headless flag, and resumability. The Index adds derived fields: children, hidden-children count, orphan flag.

### Discovery and matching

- The working directory is always taken from inside the session file, never decoded from directory names (all agents' directory encodings are lossy).
- Current-directory mode: for pi, claude, and droid, encode the normalized cwd with that agent's directory-naming rule and read only that directory as a pre-filter, then verify against the in-file cwd. `-r` requires scanning project directories whose encoded names share the prefix, then verifying. Codex is date-organized, so it always scans all files, reading only the first line for cwd.
- Global mode: scan every provider's full storage concurrently.
- Path normalization: both the current directory and each session's cwd go through clean + symlink resolution; if the session's cwd no longer exists, compare cleaned strings.
- Last-updated time: file mtime.
- User-turn count: real user messages only — excluding tool results, hook-only/system-injected messages.
- Readers must handle very long lines (Codex's first line embeds a ~20 KB system prompt; Claude files can be hundreds of KB). Use large line buffers or streaming decoding.

### Per-agent facts (as observed on the target machine)

**pi**
- Storage: `~/.pi/agent/sessions/<encoded-cwd>/<timestamp>_<uuid>.jsonl`; respect `PI_CODING_AGENT_SESSION_DIR` / `PI_CODING_AGENT_DIR`.
- Directory encoding: `--` + cwd with `/` → `-` + `--`.
- First line: `type: "session"` with `id`, `timestamp`, `cwd`.
- Title: `name` from the latest `session_info` entry; else first `message` with `role: "user"`.
- Sub-agents: the first line carries `parentSession` (path to the parent session file); parent = the id in that file name. Their `session_info` name looks like `<agent-type>#<hash>`; the part before `#` is the tag and the title falls back to the first user message.
- Resume: `pi --session <full path to session file>`. Sub-agents resume themselves.

**Claude Code**
- Storage: `~/.claude/projects/<encoded-cwd>/<uuid>.jsonl`; respect `CLAUDE_CONFIG_DIR`.
- Directory encoding: cwd with `/`, `.`, `_`, `~` (and likely other non-alphanumerics) → `-`.
- First lines are metadata (`mode`, `permission-mode`, `file-history-snapshot`); cwd, sessionId, timestamp appear on `user`/`assistant` lines.
- Title: the last `ai-title` entry; else first non-sidechain user message with text content.
- Sub-agents: `<parent-uuid>/subagents/agent-<id>.jsonl` with a sibling `.meta.json` containing `agentType`, `description`, `spawnDepth`; lines carry `isSidechain: true` and the parent `sessionId`. Parent = the enclosing session UUID. Title = `description`, tag = `agentType`. Only top-level `*.jsonl` files in a project directory are main sessions.
- Resume: `claude --resume <id>`. Sub-agent rows resume the parent.

**Codex**
- Storage: `~/.codex/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl`; respect `CODEX_HOME`.
- First line: `type: "session_meta"` with `payload.id`, `payload.cwd`, `payload.source`, `payload.thread_source`, `payload.originator`, and for sub-agents `payload.parent_thread_id`.
- Title: `thread_name` from `session_index.jsonl` (keyed by id); else first user message.
- Sub-agents: `thread_source == "subagent"`; parent = `parent_thread_id`; tag from `source.subagent` (e.g. `review`, `guardian`).
- Headless: originators other than the interactive CLI and Codex Desktop (e.g. agent SDKs, repoprompt) are treated as headless. Codex Desktop (`source: vscode`) sessions are normal.
- Resume: `codex resume <id>`. Sub-agents resume themselves.

**Factory Droid**
- Storage: `~/.factory/sessions/<encoded-cwd>/<uuid>.jsonl` (plus `<uuid>.settings.json` sidecars to ignore); respect `FACTORY_HOME_OVERRIDE`.
- Directory encoding: cwd with `/` → `-`.
- First line: `type: "session_start"` with `id`, `title`, `cwd`, and for sub-agents `callingSessionId`.
- Title: `title` unless it is `"New Session"`; else first real user message (skip hook-only messages with `visibility: "user_only"` / `hookEventName` and empty content).
- Sub-agents: presence of `callingSessionId`; parent = that ID. (`parent` on `session_start` marks a fork, not a sub-agent, and is ignored.)
- Headless: no marker for `droid exec` sessions has been found, so Droid sessions are never treated as headless.
- `FACTORY_HOME_OVERRIDE` replaces the home directory (Droid resolves it as `FACTORY_HOME_OVERRIDE ?? HOME`), so sessions live under `$FACTORY_HOME_OVERRIDE/.factory/sessions`.
- Resume: `droid --resume <id>`. Sub-agents resume themselves.

### Filtering rules

- Always hidden: sessions with zero real user messages.
- Hidden unless `--subagents`: sub-agent sessions (shown as a tree under their parent). Without the flag, parents display a "+N sub" hint.
- Hidden unless `--headless`: headless/SDK sessions.
- A sub-agent's visibility in a directory-scoped view follows its root parent's cwd, not its own. Sub-agents whose parent is missing or filtered out are shown at the top level as orphans.

### List and tree presentation

- Columns: agent tag, relative time, title (truncated), user-turn count; plus shortened cwd in `-g` mode.
- Sort: top-level by own last-updated time descending; siblings likewise.
- Tree fully expanded when `--subagents` is on; no collapse/expand in v1.

### Cache

- Single index file in the user cache directory, keyed by (path, size, mtime), carrying a schema version bumped whenever parsing logic changes.
- Codex's `session_index.jsonl` is a title source; its changes must also invalidate affected cached titles (e.g. by tracking its mtime).

### Error handling

- Parse failures and unexpected formats: skip the file, record a diagnostic visible under `--debug`.
- Missing agent binary or missing cwd: row is dimmed; Enter shows an inline error and stays in the TUI. In non-interactive modes these sessions are still listed with a resumability field.

### Platform and distribution

- macOS and Linux only (exec-based process replacement).
- Distributed via `go install`. Module path under the author's GitHub account.

## Testing Decisions

- **Seam:** the Index is the single primary test seam. Tests construct a temporary home directory containing anonymized fixture session trees for all four agents, point the Index at it (via the same storage-root overrides the real tool honors), run a list query (scope, cwd, flags), and assert on the resulting forest via golden files. This exercises providers, path normalization, filtering, parent linking, sorting, and cache behavior through one public interface.
- **Resume command construction** is asserted as data (agent binary, args, target directory) from the same records, without actually exec'ing. Process replacement itself is not tested.
- **What makes a good test:** assert only externally visible results — which sessions appear, in what order and nesting, with what titles, counts, tags, flags, and resume commands. Do not assert on internal parsing helpers or intermediate structures; providers can be refactored freely as long as Index output is unchanged.
- **Fixtures** are derived from real session files on the author's machine, anonymized and trimmed, and must cover per agent: a normal session, a titled and an untitled session, an empty/hook-only session, a headless session where applicable, sub-agents (including nested and orphaned), a session whose cwd no longer exists, and an oversized first line (Codex).
- **Cache tests** at the same seam: second query with unchanged files reuses cache; touching a file re-parses it; schema-version mismatch discards the cache; `--no-cache` bypasses it.
- **TUI:** no automated tests in v1.
- **Prior art:** none — this is a new repository.

## Out of Scope

- Windows support.
- A preview pane showing session content.
- Collapsing/expanding tree nodes in the TUI.
- A configuration file (e.g. per-agent default resume arguments).
- Subcommands beyond `list` (show, delete, rename, export, search inside content).
- A shell-function wrapper to leave the parent shell in the session's directory after exit.
- Workarounds for resuming sessions whose working directory has been deleted or moved.
- Agents beyond pi, Claude Code, Codex, and Droid (e.g. Gemini, opencode, Cursor), though the provider design must make them easy to add.
- Release automation (goreleaser, Homebrew tap).

## Further Notes

- All four agents' on-disk formats are undocumented and can change with any release; the tool is deliberately tolerant (skip and report) rather than strict.
- Droid sub-agent sessions are numerous on this machine (~a third of all Droid sessions), which is why hiding them by default matters.
- Directory pre-filter: instead of reproducing each agent's exact encoding, both the encoded directory name and the target path are reduced to letters, digits, and non-ASCII characters separated by `-`, then compared. The in-file cwd check remains the source of truth.

### Known limitations (found during implementation)

- In current-directory mode, pi and Droid only read the project directories matching the current directory. A sub-agent that ran in a different directory than its parent would not appear in the parent's tree. On the author's machine all 171 pi and Droid sub-agents share their parent's cwd.
- Sessions recorded under a symlinked path are not found when listing from the resolved real path (the reverse works). Agents normally record the physical path, so this is rare.
- Codex rollouts are parsed in full even in current-directory mode; the cache makes this a one-time cost per file.
- Tab cycles only through agents that have sessions in the current listing.
- Orphaned sub-agents are only shown with `--subagents`, consistent with sub-agents being hidden by default.
