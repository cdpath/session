# PRD: switch scope inside the picker

## Problem Statement

`session` lists sessions for one scope, chosen by flags at startup: the current directory (default), the current directory with its subdirectories (`-r`), or all directories (`-g`). When I open the picker in a project and the session I want isn't there (it was started in a subdirectory, or in another project), I have to quit, retype the command with `-r` or `-g`, and redo my agent and text filters. When the current directory has no sessions at all, the tool prints a message and exits, so the picker never opens and I can't widen the view from inside it.

## Solution

Inside the picker, pressing `s` cycles the scope: current directory → current directory with subdirectories → all directories → back to current directory. The switch is instant because every session is loaded once at startup and each scope is a filter over the same data. The agent filter, the text filter, and the selected session survive the switch. The header always says which scope is active, and the cwd column appears whenever the scope spans more than one directory. The picker opens even when the starting scope is empty, and the empty view tells me to press `s`.

Flags still choose the starting scope. Nothing is remembered between runs. The non-interactive table and JSON output behave as before.

## User Stories

### Switching scope

1. As a developer, I want to press `s` in the picker to widen the scope without quitting, so that I can find a session that was started somewhere else.
2. As a developer, I want `s` to cycle current directory → current directory with subdirectories → all directories → current directory, so that one key covers every scope and returns me to where I started.
3. As a developer, I want the help line to show `s scope`, so that I can discover the key.
4. As a developer, I want the switch to be instant, so that trying a wider scope costs nothing.
5. As a developer, I want `-r` and `-g` to still choose the scope the picker starts in, so that my existing habits and aliases keep working.
6. As a developer, I want each run to start from the scope given by its flags, not from whatever I chose last time, so that the tool stays predictable.
7. As a developer, I want `s` typed while I'm entering a filter to go into the filter text, so that I can search for words containing "s".
8. As a developer, I want `s` to work while a filter is applied (after I pressed Enter on it), so that I can widen a filtered view directly.

### What the header and columns show

9. As a developer, I want the header in the current-directory scope to read like `~/proj · all agents`, so that I know which directory I'm looking at.
10. As a developer, I want the header in the subdirectories scope to read like `~/proj w/ subdirs · all agents`, so that I know subdirectories are included.
11. As a developer, I want the header in the global scope to read `all directories · all agents`, so that I know nothing is filtered by directory.
12. As a developer, I want the agent part of the header to keep showing the current agent filter (for example `~/proj w/ subdirs · codex`), so that both filters are visible at once.
13. As a developer, I want no cwd column in the current-directory scope, so that the rows don't repeat the directory that is already in the header.
14. As a developer, I want a cwd column relative to the starting directory in the subdirectories scope (`.` for the directory itself, `./web` for a subdirectory), so that I can tell which part of the project each session belongs to without long paths.
15. As a developer, I want a cwd column with home-shortened paths (`~/Developer/foo`) in the global scope, as with `-g` today, so that I can tell projects apart.

### What survives a scope switch

16. As a developer, I want the agent filter chosen with Tab to stay the same after pressing `s`, so that I keep looking at the same agent in a wider scope.
17. As a developer, I want an applied text filter to stay applied after pressing `s`, so that I can search for a keyword and then widen until it turns up.
18. As a developer, I want the selected session to stay selected after pressing `s` when it is still visible, so that I don't lose my place.
19. As a developer, I want the selection to move to the first row when the selected session is not visible in the new scope, so that the cursor always lands on something sensible.
20. As a developer, I want the header to stay visible however many sessions the new scope contains, so that switching scope never brings back the missing-header bug.

### Agent filter

21. As a developer, I want Tab to cycle through every agent that has sessions anywhere, not just in the current scope, so that the agent cycle doesn't change when I switch scope.
22. As a developer, I want Tab to keep its current behavior of clearing the text filter, so that only the new key behaves differently.

### Empty states

23. As a developer, I want the picker to open even when the starting scope has no sessions, so that I can widen the scope from inside it.
24. As a developer, I want an empty scope to say "No sessions here · press s to widen scope", so that I know what to do next.
25. As a developer, I want an empty scope with an agent filter to say something like "No codex sessions in ~/proj · s widen scope · tab next agent", so that I know both ways out.
26. As a developer, I want the plain "No sessions found" message and an immediate exit when there are no sessions in any directory, so that I'm not dropped into a picker with nothing to widen to.

### Data correctness gained by loading everything

27. As a developer, I want a sub-agent that ran in a different directory from its parent to still appear in the parent's tree in every scope, so that trees are never incomplete.
28. As a developer, I want sessions recorded under a symlinked path to appear when I list from the resolved real path, so that symlinks don't hide sessions in either direction.

### Unchanged behavior

29. As a developer, I want the table output (stdout not a terminal) to list the same sessions for the same flags as before, so that my scripts keep working.
30. As a developer, I want `--json` output to list the same sessions for the same flags as before, so that my scripts keep working.
31. As a developer, I want the non-interactive empty message ("No sessions found in ~/proj (try -r or -g)") unchanged, so that scripts and habits keep working.
32. As a developer, I want `--subagents`, `--headless`, and `-a` to keep applying to every scope in the picker, so that the flags mean the same thing everywhere.
33. As a developer, I want Enter, `/`, quit, dimmed rows, and inline resume errors to work the same in every scope, so that the new key adds no surprises.
34. As a developer, I want a warm-cache start to stay about as fast as today (tens of milliseconds), so that the default command stays snappy.

## Implementation Decisions

### Loading

- The Index always scans every provider's full storage. Scope is no longer passed to providers; it becomes a filter applied after the forest is built (sub-agents linked, headless and empty sessions dropped, resume commands and problems computed). This is what fixes stories 27 and 28: parent links and path matching no longer depend on which project directories were read.
- Measured on the author's machine (about 680 sessions): a warm-cache start is about 30 ms for every scope; a cold start for the current directory goes from about 0.69 s to about 0.87 s, the same as a cold `-g` today.
- Consequences inside the Index: the cache always holds every session, so the logic that merges entries from directories a scoped run did not visit goes away. Providers drop the scope argument from discovery, and the directory pre-filter helpers (encoded-name matching, prefix scans) are deleted. Codex already scanned everything.
- The `-a` agent filter is applied to parsed sessions, not to the set of providers scanned, so a filtered run still rewrites a complete cache.
- Resumability problems are computed for every listed session; the picker lists all directories, so that covers every session it can show. The checks are already memoized per binary and per directory, so the cost is bounded by the number of distinct directories.

### Scope as a domain concept

- Scope has three values: current directory, current directory with subdirectories, all directories. It has a fixed cycle order for the picker.
- The Index exposes a pure scope filter: given a forest, a starting directory, and a scope, return the root sessions whose root cwd matches, with their sub-agent trees intact. Matching rules are unchanged: both paths are cleaned and symlinks are resolved when the path exists; "subdirectories" means equal to or below the starting directory. Each distinct directory is resolved once, not once per session or per key press.
- The Index list query takes a scope instead of separate global and recursive switches, and applies the same scope filter as its last step, so non-interactive output is unchanged.
- To feed the picker, the CLI runs one list query with the all-directories scope and hands the full forest to the picker together with the starting directory and the starting scope.

### Picker

- Picker configuration carries: the full forest, the starting directory, the starting scope, the home directory, and the agents for Tab. The fixed heading string and the "global" flag are replaced by values derived from the current scope.
- The Tab agent list is every agent present in the full forest, in provider display order.
- `s` is a new key binding, active whenever the user is not typing a filter. It advances the scope, re-applies the scope filter and the current agent filter, and keeps the text filter.
- Header text per scope: `<short path> · <agent>`, `<short path> w/ subdirs · <agent>`, `all directories · <agent>`.
- Row cwd column per scope: none; path relative to the starting directory (`.` or `./sub/dir`, computed from resolved paths so a symlinked starting directory still produces short relative paths); home-shortened path.
- Selection is kept by session identity (the same session record; the forest is loaded once, so records are stable). If the selected session is gone from the new item set, the first row is selected.
- With an applied text filter, the list component re-filters new items through a command. The model runs that command synchronously during the scope switch, so the selection can be restored against the filtered items, then re-applies the pagination fix before selecting.
- Empty states replace the list's default empty text whenever no row is visible (including when an applied text filter matches nothing in the new scope): with no agent filter, "No sessions here · press s to widen scope"; with an agent filter, "No <agent> sessions in <scope label> · s widen scope · tab next agent". In the all-directories scope there is nothing wider, so the hint becomes "No sessions anywhere" or "No <agent> sessions in all directories · tab next agent".

### CLI

- In the interactive path, the "No sessions found" message and exit apply only when the full forest is empty. Otherwise the picker opens, even if the starting scope is empty.
- Non-interactive paths (table, JSON) and their empty message are unchanged.

### Documentation

- PRD 0001's known limitations lose the sub-agent-in-another-directory and symlinked-path entries, and the Tab entry changes to "Tab cycles through agents that have sessions in any directory".
- The README key list gains `s`.

## Testing Decisions

- **Good tests** assert only externally visible behavior: which sessions are listed, in what order and nesting, what the header and rows say, and which session ends up selected. They do not inspect model fields or list-component internals.
- **Index seam** (existing): the current fixture-based list tests stay green, unchanged, which proves non-interactive output is unchanged. New list tests cover a pi or Droid sub-agent whose cwd differs from its parent's appearing under the parent in the current-directory scope, and a session recorded under a symlinked path appearing when listing from the real path. Prior art: the index tests and fixture writers for each agent, including the global and recursive scope test.
- **Scope filter** (new, pure): given a hand-built forest, assert the roots returned for each of the three scopes, including exact-match versus subdirectory matching, a sibling directory that only shares a name prefix, symlinked starting directories, and that sub-agent trees and hidden-sub-agent counts are kept.
- **Picker model** (existing seam): drive the model with key and window-size messages through the existing helper that also runs the returned commands, then assert on the rendered view. Cover: the `s` cycle and its header text, the cwd column per scope (absent, relative, home-shortened), keeping the agent filter, keeping an applied text filter, keeping the selection or falling back to the first row, `s` typed while entering a filter, Tab cycling through agents absent from the current scope, both empty-state messages, and the header staying on the first line when a scope switch goes from one page to several. Prior art: the two missing-header regression tests.
- **CLI**: the interactive path is not tested automatically. After implementation, a manual check runs in a fake HOME (never the real one, to avoid resuming a real session by accident).

## Out of Scope

- Toggling sub-agents or headless sessions from inside the picker.
- Remembering the last scope between runs, or a configuration file.
- Changing the starting scope rules or adding new flags.
- Scopes other than the three listed (for example "the git repository root" or a chosen directory).
- Reloading sessions from disk while the picker is open.
- Changing what Tab does to the text filter.

## Further Notes

- `s` is not bound by the list component's default key map, so it doesn't clash with navigation or filtering keys.
- "w/" is the intended spelling in the subdirectories header.
- Codex rollouts are still parsed in full on first sight; the cache makes that a one-time cost per file, as before.
