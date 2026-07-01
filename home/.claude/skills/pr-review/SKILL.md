---
name: pr-review
description: Address unresolved PR review comments — reads plan.toml (populated by `work sync`), plans fixes, implements them, and resolves threads
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(work *), Bash(yq *), Bash(gh pr view:*), Bash(gh-pr-thread-resolve:*), Read, Grep, Glob, Edit, Write, AskUserQuestion
argument-hint: [pr-number]
---

# PR Review

Address unresolved PR review comments in one conversation with three stops. The
canonical source for unresolved comments is `plan.toml`'s `[[pr.comment]]`
array, populated by `work sync`.

## Input

`$ARGUMENTS` is an optional PR number. The current-worktree default is the
common case:

- If no arg: the current worktree's PR is used. Run `work sync` first to refresh.
- If a number is given: locate the worktree that has that PR in its `plan.toml`.

## Setup

1. Determine the worktree:
   - If `$ARGUMENTS` is empty, use the current worktree (`work list -w` to
     confirm you're in one).
   - Otherwise, find the worktree whose `plan.toml` contains that PR number.

2. Refresh from GitHub:
   ```
   cd <worktree> && work sync
   ```
   This populates `[[pr.comment]]` in the worktree's `plan.toml` with every
   unresolved review thread — one entry per comment. Each has:
   - `thread` — GraphQL thread ID (for later resolve)
   - `author`, `source` (file:line), `comment` — original review content
   - `status` — starts as `open`
   - `plan`, `reply`, `fix_ref` — populated by us

## Phase 1 — Plan

1. Read the worktree's `plan.toml`. Iterate `[[pr.comment]]` entries.

2. For each comment with empty `plan`, draft one:
   - Read the file at `source` (file:line) and understand the surrounding code.
   - Write a specific fix — name functions, variables, and the exact change.
   - No em dashes, no filler. One or two short sentences.

3. Present the plan in the conversation as a table:

   ```
   thread | source                   | plan
   -------|--------------------------|-------------------------------
   T_ABC1 | src/handler.go:42        | add nil check on Fetch() result
   T_DEF2 | src/store.go:88          | rename local `x` → `count`
   ...
   ```

4. **Stop.** Tell the user how many items are pending. The user reviews the
   proposed plans (and may open `plan.toml` in their editor to tweak them or
   mark items `status = "done"` if they're already addressed).

## Phase 2 — Implement

Reached when the user says "go" or "implement".

1. Re-read `plan.toml` (they may have edited it).

2. For each `[[pr.comment]]`:
   - `status = "done"` — already addressed, skip.
   - `status = "open"` — implement the fix described in `plan`.
     - Read the file, make the edit. **Never commit or push.**
     - After the edit succeeds, update the comment's `status` to `"done"` in
       `plan.toml` and set `fix_ref` to a short description (e.g., "nil-check
       in Fetch caller"). Actual commit hash is filled in during Phase 3.

3. Print a summary table:
   ```
   file                          | change
   ------------------------------|----------------------------------
   src/handler.go                | add nil check to Fetch caller
   src/store.go                  | rename x → count
   ```

4. **Stop.** Tell the user to `git diff`, commit, and push. Wait for them to
   confirm.

## Phase 3 — Resolve

Reached when the user says "resolved" or "posted".

1. Get the latest commit hash:
   ```
   git rev-parse --short HEAD
   ```
   (or `gh pr view <n> --json commits --jq '.commits[-1].oid[0:7]'` if the
   changes were force-pushed.)

2. For every `[[pr.comment]]` with `status = "done"`:
   - Build the reply: `<short description> (<hash>)`. Update the `reply` field
     in `plan.toml`.
   - Resolve the thread:
     ```
     gh-pr-thread-resolve "<thread-id>" "<reply text> (<hash>)"
     ```

3. Batch the resolves so a single failure doesn't stop the rest:
   ```bash
   failed=0
   if ! gh-pr-thread-resolve "T_ABC1" "add nil-check in Fetch caller (abc1234)"; then
     echo "FAIL: T_ABC1"; failed=$((failed + 1))
   fi
   if ! gh-pr-thread-resolve "T_DEF2" "rename x → count (abc1234)"; then
     echo "FAIL: T_DEF2"; failed=$((failed + 1))
   fi
   if [ "$failed" -gt 0 ]; then
     echo "$failed thread(s) failed to resolve"
     exit 1
   fi
   ```

4. Report how many threads were resolved vs skipped. The next `work sync` will
   drop the resolved comments from `plan.toml` (sync only surfaces unresolved
   threads).

## Editing plan.toml

**Always use `yq` for TOML edits — never hand-edit or use `sed`.** `yq -p toml
-o toml` round-trips the file, keeping structure sound.

Examples (adjust the index):

```bash
# read the current plan/status of a comment
yq -p toml '.pr.comment[2].status' plan.toml

# set status/plan/reply/fix_ref on a comment (by 0-based array index)
yq -p toml -o toml -i '.pr.comment[2].status = "done"' plan.toml
yq -p toml -o toml -i '.pr.comment[2].plan   = "add nil-check to Fetch caller"' plan.toml
yq -p toml -o toml -i '.pr.comment[2].reply  = "add nil-check in Fetch caller (abc1234)"' plan.toml
yq -p toml -o toml -i '.pr.comment[2].fix_ref = "abc1234"' plan.toml

# find the index of a comment by thread id
yq -p toml '.pr.comment | to_entries | map(select(.value.thread == "T_ABC1")) | .[0].key' plan.toml
```

## Notes

- **Never write to `plan.md`.** That's the freeform scratchpad — pr-review's
  data lives in `plan.toml`'s `[[pr.comment]]` array.
- Editing `plan.toml` from the skill is fine, but only through `yq -i`.
  Preserve unrelated fields; only touch the `[[pr.comment]]` entries you're
  processing.
- Replies must be terse. No em dashes, no filler. One short sentence plus
  commit hash.
- If a comment is already addressed in the branch code (nothing to change),
  set `status = "done"` and write an appropriate `reply` in Phase 1 — it'll
  still get resolved in Phase 3.
