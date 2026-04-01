---
name: pr-plan
description: Read PR review comments and create a PLAN.md in a worktree with drafted fixes
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh *), Bash(git *), Bash(mkdir *), Bash(ls *), Read, Grep, Write
argument-hint: <pr-number>
---

# PR Plan

Create a plan for addressing PR review comments.

## Input

`$ARGUMENTS` is a PR number.

## Steps

1. **Resolve repo info:**
   ```
   gh pr view $ARGUMENTS --json number,headRefName,baseRefName,url,repository
   ```
   Extract the PR number, head branch, and `owner/repo`.

2. **Fetch review comments:**
   ```
   gh api graphql -f query='
     query($owner: String!, $repo: String!, $number: Int!) {
       repository(owner: $owner, name: $repo) {
         pullRequest(number: $number) {
           reviewThreads(first: 100) {
             nodes {
               id
               isResolved
               comments(first: 10) {
                 nodes {
                   author { login }
                   body
                   path
                   line
                   diffHunk
                 }
               }
             }
           }
         }
       }
     }' -F owner=OWNER -F repo=REPO -F number=$ARGUMENTS
   ```
   Skip threads that are already resolved.

3. **Create the worktree** using the same convention as `~/dotfiles/home/bin/work`:
   - Repo name = basename of the git toplevel
   - Path: `~/w/{repo}/pr-$ARGUMENTS`
   - Branch off the PR's head branch:
     ```
     git fetch origin <head-branch>
     git worktree add ~/w/{repo}/pr-$ARGUMENTS origin/<head-branch>
     ```
   - If the worktree already exists, skip creation and reuse it. Warn the user that PLAN.md will be overwritten.

4. **Write `PLAN.md`** at the worktree root with this exact format:

   ```markdown
   ---
   pr: <number>
   repo: <owner/repo>
   branch: <head-branch>
   worktree: ~/w/<repo>/pr-<number>
   ---

   ## Handle nil error

   | key      | value                |
   | -------- | -------------------- |
   | thread   | <thread-id>          |
   | file     | src/handler.go:42    |
   | author   | reviewer-name        |
   | status   | pending              |
   | commit   |                      |

   ### Comment

   > Original review comment body, blockquoted.
   > Can be multiple lines.

   ### Reply

   Add a nil check on the return value of `Fetch()` and wrap
   the error with context before returning.
   ```

   The `##` title should be 2-3 words summarizing what needs to change (not the thread ID). The metadata table should have nicely aligned columns. The `### Reply` section is your drafted proposed fix — read the relevant code in the worktree to be specific about functions, variables, and the exact change.

   Repeat for each unresolved thread.

6. **Done.** Tell the user:
   - Where the PLAN.md is
   - How many comments were planned
   - Remind them to review/edit the plan, mark any items `status: skip`, then run `/pr-edit` from the worktree directory
