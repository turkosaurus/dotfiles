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

## State machine

Each `[[pr.comment]]` moves through three states, one per phase:

| Status | Meaning | Set by |
|---|---|---|
| `open` | Synced from GitHub; no plan yet. | `work sync` (Setup) |
| `pending` | Plan (and usually reply) drafted; proposal — user may dissent. | Phase 1 |
| `done` | Fix in code (or confirmed nothing to fix), reply finalized. | Phase 2 |

Phase 1 is a **proposal** — nothing is `done` until Phase 2 confirms.
Write the reply as soon as you can (usually Phase 1); Phase 2 may
refine it if implementation diverges. Phase 3 appends the commit
hash so every posted reply ends with a ref, e.g.
`added nil-check on Fetch caller abc1234`.

Phase 3 resolves the thread on GitHub; the comment then drops from
`plan.toml` on the next `work sync`.

## Phase 1 — Plan

1. Read the worktree's `plan.toml`. Iterate `[[pr.comment]]` entries
   with `status = "open"`.

2. For each, read the file at `source` (file:line) to understand the
   surrounding code, then:

   - Draft a specific fix plan — name functions, variables, and the
     exact change. No em dashes, no filler. If the comment is already
     addressed in the branch code, the plan should say so
     ("already fixed in <ref>; reply only") — Phase 2 will confirm
     without editing.
   - Draft a terse reply now (one short sentence). Refine in Phase 2
     if the actual implementation diverges.
   - **Persist by calling `/work update`.** Always status → `pending`:

     ```
     /work update "<plan text>" --to pr-comment=<thread-id> \
       --status pending --reply "<short reply>"
     ```

     Parse the machine receipt (`status=ok` or `status=ambiguous`).
     On `ok`, move to the next comment; on `ambiguous`, surface the
     `suggest=` line to the user and stop.

3. Present the plan in the conversation as a **human receipt** table
   (per AGENTS.md "structured output"):

   ```
   wrote:
     pr[0].comment[0].plan   ← 'add nil check on Fetch()...'
     pr[0].comment[0].reply  ← 'added nil-check on Fetch caller'
     pr[0].comment[0].status ← pending
     pr[0].comment[1].plan   ← 'already fixed in retry loop'
     pr[0].comment[1].status ← pending
   ```

   Don't restate what each comment says — the write is the
   description. The user can `cat plan.toml` for full detail.

4. **Stop.** Tell the user how many items are pending. The user
   reviews the proposed plans (and may open `plan.toml` in their
   editor to tweak them, or dissent — nothing is `done` yet).

## Phase 2 — Implement

Reached when the user says "go" or "implement".

1. Re-read `plan.toml` (they may have edited it).

2. For each `[[pr.comment]]` with `status = "pending"`:
   - If the plan calls for a code change, read the file and make
     the edit. **Never commit or push.**
   - If the plan says "already fixed", verify it still holds — no
     edit needed, just confirm.
   - If the implementation diverged from the plan, refine the
     `reply` to match what actually changed.
   - Mark the comment done via `/work update`:

     ```
     /work update --to pr-comment=<thread-id> --status done \
       --fix-ref "<short description>" \
       [--reply "<refined reply>"]
     ```

3. Print a summary table:
   ```
   file                          | change
   ------------------------------|----------------------------------
   src/handler.go                | add nil check to Fetch caller
   src/store.go                  | rename x → count
   ```

4. **Stop.** Tell the user to `git diff`, commit, and push. Wait for
   them to confirm.

## Phase 3 — Resolve

Reached when the user says "resolved" or "posted".

1. Get the latest commit hash:
   ```
   git rev-parse --short HEAD
   ```
   (or `gh pr view <n> --json commits --jq '.commits[-1].oid[0:7]'` if the
   changes were force-pushed.)

2. For every `[[pr.comment]]` with `status = "done"`:
   - Append ` <hash>` (space, no parens) to the existing `reply` if
     it isn't already there. Every reply must end with a commit ref
     — e.g. `added nil-check on Fetch caller abc1234`. Persist via
     `/work update --to pr-comment=<thread-id>` with
     `--reply "<existing reply> <hash>"`.
   - Resolve the thread:
     ```
     gh-pr-thread-resolve "<thread-id>" "<reply text> <hash>"
     ```

3. Batch the resolves so a single failure doesn't stop the rest:
   ```bash
   failed=0
   if ! gh-pr-thread-resolve "T_ABC1" "added nil-check on Fetch caller abc1234"; then
     echo "FAIL: T_ABC1"; failed=$((failed + 1))
   fi
   if ! gh-pr-thread-resolve "T_DEF2" "renamed x → count abc1234"; then
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
  ending with the commit hash (space-separated, no parens):
  `added nil-check on Fetch caller abc1234`.
- If a comment is already addressed in the branch code (nothing to
  change), set `status = "done"` and write an appropriate `reply` in
  Phase 1 — Phase 3 still appends the hash and resolves the thread.
