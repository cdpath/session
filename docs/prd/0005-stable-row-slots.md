# PRD: stable row slots across scopes

## Problem Statement

Pressing `s` cycles scopes and the row shape changes with it: in the current-directory scope the repo slug and the cwd slots vanish, so rows visually jump as columns appear and disappear. The hiding rules were also inconsistent in principle: the agent tag stays visible even when constant (a single-agent Tab view shows the same tag on every row), while the slug was hidden for being constant — and the "it would be identical on every row" premise breaks for sub-agents that ran in a different repository, whose project identity was invisible in the Here scope altogether.

## Solution

Two stable slots on every row, in every scope:

- The **repo slug** becomes an identity column: always shown at the top level, in every scope, in the same blue. Suppression stays a tree-level rule: a sub-agent row hides slug and cwd when they match its parent's, exactly as before.
- The **cwd** becomes an always-present context column with one unified form: inside the starting directory → path relative to it (`.` for the directory itself, `./sub/dir` below it); outside it → home-shortened path. The Here scope's "no cwd column" rule is dropped; Here now renders exactly like the subdirectories scope.

The header still shows the starting directory, so the one-character `.` repetition in the Here scope costs nothing. Non-interactive output (table, JSON) is unchanged.

This PRD supersedes: PRD 0002 story 13 ("no cwd column in the current-directory scope") and its "Row cwd column per scope" line, and PRD 0004 story 8 ("no slug in the current-directory scope") together with its "Visibility follows the cwd rules exactly" clause. PRD 0001 story 15's shortened-cwd-in-`-g` rule is untouched: it governs the non-interactive table, which this PRD does not change.

## User Stories

1. As a developer, I want the repo slug visible in every scope, including the current-directory scope, so that a row's project identity never disappears when I cycle scopes.
2. As a developer, I want every row to keep the same slot layout across scope switches, so that the list does not visually jump while I press `s`.
3. As a developer, I want the current-directory scope to show `.` as the cwd, so that always-present cwd costs one character instead of a repeated full path.
4. As a developer, I want a sub-agent that ran in a different directory or repository to show its own cwd and slug in every scope (when they differ from its parent's), so that its identity is never invisible.
5. As a developer, I want sub-agent rows to keep hiding cwd and slug when they match the parent's, so that tree rows only call out differences.
6. As a developer, I want the header to keep showing the starting directory, so that `.` rows still have an anchor elsewhere on screen.
7. As a developer, I want `/` to match the slug and cwd in every scope, so that filtering behaves the same everywhere.
8. As a developer, I want width pressure to keep dropping the slug before the cwd, so that narrow terminals degrade as they do today.
9. As a developer, I want the agent tag, colors, turns, and recency columns untouched, so that only the two slots in question change.
10. As a developer, I want table and JSON output to behave exactly as before, so that my scripts keep working.

## Implementation Decisions

- Picker repo label: the Here-scope gate is removed; the slug renders in all three scopes. Tree-level differ-from-parent suppression is unchanged.
- Picker cwd: the Here branch is removed; Here and the subdirectories scope share one rule — inside the starting directory → relative path (`.` or `./sub/dir`), outside → home-shortened path. The global scope keeps home-shortened paths. Net effect: the cwd function has two paths (relative-when-inside-and-scoped, short-path-otherwise) instead of three.
- The filter value, precomputed per rebuild, includes the slug label and cwd in every scope; the scope gate disappears with the delegate's. The filter value carries each session's raw cwd path, as before — typing the displayed form (`.` or `~/…`) does not match.
- Width-pressure fallback order is unchanged: drop the slug first, then the cwd.
- Non-interactive paths are untouched: the table keeps showing CWD and REPO columns only in global mode, and JSON keeps the `repo` field as is.
- No changes to the index, cache, or reposlug packages.

## Testing Decisions

- All assertions through the existing picker rendering seam (tui tests): flip the Here-scope hides-slug / hides-cwd expectations into shows-slug (colored) and shows-`.`; sub-agent rows hide matching labels and show differing ones in every scope; a row's slot layout is identical across scope switches.
- No new seams: the index and CLI are unchanged.

## Out of Scope

- Changes to table or JSON output.
- Grouping or de-duplicating repeated slugs across rows.
- Header changes.
