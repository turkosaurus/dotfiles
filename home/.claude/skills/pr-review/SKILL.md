---
name: pr-review
description: Fetch unresolved PR comments, plan fixes, implement them, and resolve threads
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh pr view:*), Bash(gh-pr-threads:*), Bash(gh-pr-thread-resolve:*), Read, Grep, Glob, Edit, Write, AskUserQuestion
argument-hint: [pr-number]
---

# PR Review

Address unresolved PR review comments in one conversation with three stops.

## Input

`$ARGUMENTS` is an optional PR number. If omitted, detect the PR for the
current branch:

```
gh pr view --json number,headRefName,baseRefName,url,headRepository,headRepositoryOwner
```

If a number is given, pass it explicitly:

```
gh pr view $ARGUMENTS --json number,headRefName,baseRefName,url,headRepository,headRepositoryOwner
```

Extract the PR number, head branch, and `owner/repo`.

Compute the worktree path:
- `slug` = headRefName with `/` replaced by `-`
- `worktree` = `~/w/{headRepository.name}/{slug}`

If the worktree directory doesn't exist, fall back to the current git toplevel.

All file reads and edits happen inside the worktree.

## Phase 1 — Plan

1. Fetch unresolved review threads:
   ```
   gh-pr-threads <owner> <repo> <number>
   ```

2. For each unresolved thread, read the referenced file in the worktree and understand the surrounding code.

3. If `{worktree}/plan.md` already exists, **append** the PR review section
   below a `---` separator (preserving any existing content above it).
   Otherwise create the file. Write the PR review section with this format:

   ```markdown
   ---
   pr: <number>
   repo: <owner/repo>
   branch: <head-branch>
   ---

   ## Handle nil error

   | key    | value             |
   | ------ | ----------------- |
   | thread | <thread-id>       |
   | file   | src/handler.go:42 |
   | author | reviewer-name     |
   | status | pending           |

   ### Comment

   > Original review comment body, blockquoted.

   ### Plan

   Add a nil check on the return value of `Fetch()` and wrap
   the error with context before returning.
   ```

   Rules:
   - `##` title: 2-3 words summarizing the change.
   - Aligned columns in the metadata table.
   - `### Plan`: a specific proposed fix — name functions, variables, and the exact change.
   - If the thread is already addressed in the branch code, set `status: skip` and explain why.

4. **Stop.** Tell the user how many items are pending vs skip, and wait.
   The user will review `plan.md`, may edit plans or mark items `status: skip`, then say "go" or "implement."

## Phase 2 — Implement

1. Re-read `{worktree}/plan.md` (the user may have edited it).

2. For each `##` section:
   - `skip` or `done` — skip silently.
   - `pending` — implement the fix described in `### Plan`.
     - Read the file, make the edit. **Never commit or push.**
     - Update `status` to `done` in `plan.md`.

3. After all sections, print a summary table:

   ```
   file                          | change
   ------------------------------|----------------------------------
   db/queries/game_cards.sql     | add game_id to GameCardMove WHERE
   actions.go                    | validate flip card ownership
   ```

4. **Stop.** Tell the user to `git diff`, commit, and push. Wait for them to confirm.

## Phase 3 — Resolve

1. Get the latest commit hash (use the PR number resolved in Phase 1):
   ```
   gh pr view <number> --json commits --jq '.commits[-1].oid[0:7]'
   ```

2. Collect every `thread` value from `done` sections, then resolve them
   all in **one** Bash call:

   ```bash
   failed=0
   for tid in <thread-id-1> <thread-id-2> …; do
     if ! gh-pr-thread-resolve "$tid" "addressed with <hash>"; then
       echo "FAIL: $tid"
       failed=$((failed + 1))
     fi
   done
   if [ "$failed" -gt 0 ]; then
     echo "$failed thread(s) failed to resolve"
     exit 1
   fi
   ```

3. Remove only the `## PR review (#<number>)` section (and its `---` separator)
   from `{worktree}/plan.md`. If this was the only content, delete the file.

4. **Done.** Report how many threads were resolved vs skipped.
