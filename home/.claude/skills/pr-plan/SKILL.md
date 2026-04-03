---
name: pr-plan
description: Read unresolved PR review comments and create PR<number>.md in the repo root with drafted fixes
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh *), Bash(git *), Read, Grep, Glob, Write
argument-hint: <pr-number>
---

# PR Plan

Create a plan for addressing PR review comments.

## Input

`$ARGUMENTS` is a PR number.

## Steps

1. **Resolve repo info:**
   ```
   gh pr view $ARGUMENTS --json number,headRefName,baseRefName,url,headRepository,headRepositoryOwner
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

3. **Write `PR$ARGUMENTS.md`** at the git toplevel with this exact format:

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
   > Can be multiple lines.

   ### Reply

   Add a nil check on the return value of `Fetch()` and wrap
   the error with context before returning.
   ```

   The `##` title should be 2-3 words summarizing what needs to change (not the thread ID). The metadata table should have nicely aligned columns. The `### Reply` section is your drafted proposed fix — read the relevant code to be specific about functions, variables, and the exact change.

   Repeat for each unresolved thread.

4. **Done.** Tell the user:
   - Where the file is
   - How many comments were planned
   - Remind them to review/edit the plan, mark any items `status: skip`, then run `/pr-update $ARGUMENTS`
