# PRD: expand sub-agents inside the picker

## Problem Statement

Sub-agent sessions are hidden by default, and a parent row only says "+3 sub". To see what those sub-agents were, I have to quit the picker and start it again with `--subagents`, which then expands every tree in the list at once. That is all or nothing: either I see no sub-agents, or I see all of them and the list becomes long. There is also no quick way to find the sessions that spawned sub-agents at all.

## Solution

Inside the picker, a row with sub-agents can be expanded in place: `→` opens it one level, `←` closes it again. Nested sub-agents expand the same way, like a file tree. Pressing `f` narrows the list to sessions that have sub-agents, with every tree open and orphaned sub-agents shown, which is what `--subagents` gives today but limited to the sessions that matter. Pressing `f` again restores the normal list and the expansion state I had.

`--subagents` still exists: it controls table and JSON output, and in the picker it means "start with everything expanded".

## User Stories

### Expanding and collapsing

1. As a developer, I want to press `→` (or `l`) on a row with sub-agents to show its sub-agents beneath it, so that I can see what a session delegated without restarting the tool.
2. As a developer, I want `→` to open one level at a time, so that a deep tree doesn't flood the list.
3. As a developer, I want a sub-agent row that has its own sub-agents to show its own "+N sub" and expand with `→`, so that nested trees work like a file tree.
4. As a developer, I want `←` (or `h`) on an expanded row to collapse it, so that I can tidy the list again.
5. As a developer, I want `←` on a sub-agent row to move the cursor to its parent and collapse the parent, so that I can back out of a tree in one key.
6. As a developer, I want `→` on a row without sub-agents to do nothing, so that pressing it by mistake is harmless.
7. As a developer, I want collapsing a parent to hide all its descendants, so that nothing is left dangling.
8. As a developer, I want a collapsed row to keep showing "+3 sub" and an expanded row to show "−3 sub", so that I can tell at a glance which rows are open.
9. As a developer, I want expanded sub-agents drawn with the existing tree connectors (├─ / └─), so that they look the same as with `--subagents`.
10. As a developer, I want sub-agents within a tree sorted by last-updated time, newest first, as they are today, so that ordering is consistent.

### Showing only sessions with sub-agents

11. As a developer, I want `f` to show only sessions that have sub-agents, so that I can find the work that delegated to other agents.
12. As a developer, I want `f` to expand every tree it shows, so that I can see all the sub-agents immediately.
13. As a developer, I want `←` and `→` to keep working while `f` is on, so that I can still collapse trees I'm not interested in.
14. As a developer, I want orphaned sub-agents (whose parent is missing or filtered out, for example a headless parent) to appear at the top level marked "(orphan)" while `f` is on, so that no sub-agent is unreachable.
15. As a developer, I want orphaned sub-agents hidden when `f` is off, so that the default list shows only conversations I started.
16. As a developer, I want pressing `f` again to restore the normal list with the rows I had expanded before, so that the filter doesn't lose my place.
17. As a developer, I want the header to end in "· with sub-agents" while `f` is on, so that I know the list is filtered.
18. As a developer, I want an empty `f` view to say "No sessions with sub-agents here · s widen scope · f show all", so that I know both ways out.
19. As a developer, I want that empty message to read "No sessions with sub-agents anywhere · f show all" in the all-directories scope, since there is nothing wider.

### Paging

20. As a developer, I want PgDn (fn+↓ on a Mac), `d`, and `ctrl+f` to go to the next page, so that paging still works now that `→`, `l`, and `f` have new meanings.
21. As a developer, I want PgUp (fn+↑), `u`, `b`, and `ctrl+b` to go to the previous page, so that paging still works now that `←` and `h` have new meanings.
22. As a developer, I want the help line to show "→ expand" and "f subagents", and the full help (`?`) to show the new paging keys, so that I can discover them.

### Interaction with other picker features

23. As a developer, I want the expanded state of each session to survive `s` (scope), Tab (agent), and `f`, so that switching views doesn't collapse my trees.
24. As a developer, I want the `f` state to survive `s` and Tab, so that I can look for sessions with sub-agents across scopes and agents.
25. As a developer, I want an applied text filter and the selected session to survive `f`, as they survive `s`, so that toggling it costs nothing.
26. As a developer, I want Tab to keep clearing the text filter, as it does today, so that only the new keys behave differently.
27. As a developer, I want `/` to search only the rows currently visible, so that searching behaves predictably.
28. As a developer, I want `→` and `←` to keep working with a text filter applied, and the filter re-applied to the new rows, so that expanding is never blocked.
29. As a developer, I want the status bar count to be the number of visible rows, including expanded sub-agents, so that it matches what I see.
30. As a developer, I want Enter on a sub-agent row to behave as today (Claude sub-agents resume their parent, others resume themselves), so that resuming is unchanged.
31. As a developer, I want dimmed rows and inline resume errors to work the same on sub-agent rows, so that nothing new surprises me.

### Flags and output

32. As a developer, I want `--subagents` to start the picker with every tree expanded and orphans shown, so that the flag keeps its meaning.
33. As a developer, I want table and JSON output to behave exactly as before with and without `--subagents`, so that my scripts keep working.

## Implementation Decisions

### Index

- No change. The picker asks the Index for the full forest with sub-agents linked and orphans included (the same query `--subagents` makes today), across all directories as since PRD 0002. Headless filtering and scope filtering are unchanged.
- Table and JSON output keep calling the Index with the user's own `--subagents` setting, so they still get either collapsed parents with a hidden-sub-agent count or full trees.

### Picker

- The picker owns the collapse state: a set of expanded sessions, keyed by session identity, plus an "expand all" mode used by `f` and by `--subagents`.
- Picker configuration gains a flag for starting with everything expanded (set by `--subagents`).
- Rows are produced by walking the scoped, agent-filtered forest and descending only into expanded sessions. Tree connectors come from the existing flattening logic, applied to the visible part of the tree.
- The sub-agent count on a row is the number of all descendants, computed by the picker from the tree. Collapsed rows show "+N sub", expanded rows "−N sub". The count is unaffected by text filtering.
- Orphans (top-level sub-agent sessions marked orphan by the Index) are listed only while `f` is on or the picker was started with `--subagents`.
- `f` mode:
  - lists only roots with at least one sub-agent, plus orphans;
  - expands every listed tree, then lets `←`/`→` collapse and expand within that view;
  - keeps the normal-mode expanded set aside and restores it when `f` is turned off.
- `←` on an expanded row collapses it; on a sub-agent row it selects the parent and collapses it; otherwise it does nothing. Collapsing also closes everything below, so reopening shows one level again. `→` on a collapsed row with sub-agents expands one level; otherwise it does nothing.
- Every change to the row set (expand, collapse, `f`, `s`, Tab) goes through the same rebuild path PRD 0002 introduced: re-run an applied text filter synchronously, re-apply the pagination fix, then restore the selection by session identity, falling back to the first row.
- Key map: next page is PgDn, `d`, `ctrl+f`; previous page is PgUp, `u`, `b`, `ctrl+b`. `→`/`l` and `←`/`h` are taken by expand/collapse, `f` by the sub-agent filter. The short help gains "→ expand • f subagents"; the full help (`?`) lists the new paging keys and collapse. Paging stays out of the short help so it doesn't push "q quit • ? more" off narrower terminals.
- Header: "<scope label> · <agent>", with " · with sub-agents" appended while `f` is on.
- Empty hint while `f` is on and nothing is listed: "No sessions with sub-agents here · s widen scope · f show all", or "No sessions with sub-agents anywhere · f show all" in the all-directories scope (matching PRD 0002's "No sessions anywhere"). Other empty hints are unchanged.

### CLI

- The interactive path requests sub-agents and orphans from the Index regardless of `--subagents`, and passes `--subagents` to the picker as "start expanded".
- Non-interactive paths are unchanged.

### Documentation

- The README key list gains `→`/`←`, `f`, and the new paging keys, and describes `--subagents` as the expanded starting state in the picker.

## Testing Decisions

- **Good tests** assert only externally visible behavior: which rows the view shows, in what order and indentation, what the header, row markers, help line, and empty hint say, and which row is selected. They do not inspect the collapse set or list-component internals.
- **Seam:** the picker model only, the existing seam. Tests drive the model with key and window-size messages through the existing helper that also runs returned commands, then assert on the rendered view. The Index is unchanged, so its fixture tests stay as they are and prove table and JSON output are unchanged.
- **Cases:**
  - `→` expands one level, nested rows expand further, and `→` on a leaf does nothing;
  - `←` collapses an expanded row, jumps from a sub-agent to its parent and collapses it, and collapses all descendants;
  - the "+N"/"−N" markers;
  - `f` lists only parents and orphans, all expanded, with the header suffix; turning `f` off restores the earlier expansion;
  - `f` hides parents without sub-agents and shows orphans; orphans are hidden when `f` is off;
  - the expanded state and `f` survive `s` and Tab;
  - an applied text filter survives `f`, and `→` with a filter applied re-filters the new rows;
  - the new paging keys move pages and `l`/`h`/`f` no longer do;
  - the `--subagents` start state;
  - the `f` empty hint in a narrow scope and in all directories;
  - the header stays on the first line when expanding turns a one-page list into several pages.
- **Prior art:** the scope-switching and missing-header tests in the picker test suite, and their row and header helpers.
- **Manual check:** a fake HOME with nested sub-agents and an orphan, driven in a pseudo-terminal, never the real home.

## Out of Scope

- Searching inside collapsed sub-agents, or auto-expanding a parent when a search matches a hidden sub-agent.
- A key to expand or collapse everything outside `f` mode.
- Toggling headless sessions inside the picker.
- Changing table or JSON output, or the meaning of `--subagents` there.
- Remembering expansion between runs.
- Any change to how the Index links sub-agents or detects orphans.

## Further Notes

- The list component binds `→`, `l`, `f`, `←`, `h` to paging by default; those bindings are removed, which is why paging moves to PgUp/PgDn and the vim/less-style keys.
- Moving down past the last row of a page already continues onto the next page, and `g`/`G` jump to the first and last row, so paging keys are rarely needed.
- "−N sub" uses a minus sign to mirror "+N sub".
