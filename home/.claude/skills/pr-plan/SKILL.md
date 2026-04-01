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

   ## <thread-id>
   - file: <path>:<line>
   - author: <login>
   - comment: |
       <comment body, indented>
   - action: <your drafted suggestion for what to do>
   - status: pending
   - commit:
   ```

   Repeat the `## <thread-id>` section for each unresolved thread.

5. **Draft the `action:` field** for each comment. Read the relevant code in the worktree to understand context. Be specific — name the function, the variable, the exact change. Keep it to one or two lines.

6. **Done.** Tell the user:
   - Where the PLAN.md is
   - How many comments were planned
   - Remind them to review/edit the plan, mark any items `status: skip`, then run `/pr-edit` from the worktree directory
