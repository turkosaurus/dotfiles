---
name: pr-review
description: Address unresolved PR review comments — reads plan.toml (populated by `work sync`), plans fixes, implements them, and resolves threads
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(work *), Bash(gh pr view:*), Bash(gh-pr-thread-resolve:*), Read, Grep, Glob, Edit, Write, AskUserQuestion
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
   - Read the file at `source` (file:line) and understand the
     surrounding code.
   - Write a specific fix — name functions, variables, and the exact
     change. No em dashes, no filler.
   - **Persist by calling `/work update`** so structure and
     formatting are handled centrally:

     ```
     /work update "<plan text>" --to pr-comment=<thread-id>
     ```

     Parse the machine receipt (`status=ok` or `status=ambiguous`).
     On `ok`, move to the next comment; on `ambiguous`, surface the
     `suggest=` line to the user and stop.
   - If the comment is already addressed in the branch code (nothing
     to change), instead:

     ```
     /work update "<short reply>" --to pr-comment=<thread-id> --status done --reply "<short reply>"
     ```

     Phase 3 will still resolve it.

3. Present the plan in the conversation as a **human receipt** table
   (per AGENTS.md "structured output"):

   ```
   wrote:
     pr[0].comment[0].plan  ← 'add nil check on Fetch()...'
     pr[0].comment[1].plan  ← 'rename local x → count'
   ```

   Don't restate what each comment says — the write is the
   description. The user can `cat plan.toml` for full detail.

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
     - After the edit succeeds, mark the comment done via
       `/work update`:

       ```
       /work update --to pr-comment=<thread-id> --status done --fix-ref "<short description>"
       ```

       Actual commit hash is filled in during Phase 3.

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
   - Build the reply: `<short description> (<hash>)`. Persist via
     `/work update --to pr-comment=<thread-id>` with
     `--reply "<text> (<hash>)"`.
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

## Notes

- **Never write to `plan.md`.** That's the freeform scratchpad —
  pr-review's data lives in `plan.toml`'s `[[pr.comment]]` array.
- Replies must be terse. No em dashes, no filler. One short sentence
  plus commit hash.
- If a comment is already addressed in the branch code (nothing to
  change), set `status = "done"` and write an appropriate `reply` in
  Phase 1 — it'll still get resolved in Phase 3.
