---
name: pr-update
description: Verify push, resolve PR comment threads on GitHub, and clean up the worktree
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh *), Bash(git *), Bash(rmdir *), Read
argument-hint: <pr-number>
---

# PR Update

Resolve addressed PR comment threads and clean up.

## Input

`$ARGUMENTS` is a PR number.

## Steps

1. **Find the worktree.** Derive the repo name from `git rev-parse --show-toplevel` (use the basename of any existing worktree or the main repo). The worktree path is `~/w/{repo}/pr-$ARGUMENTS`. If it doesn't exist, tell the user and stop.

2. **Read `PLAN.md`** from the worktree.

3. **Fail fast: verify pushed.** For each section with `status: done`, check that the commit hash exists on the remote:
   ```
   git -C <worktree> branch -r --contains <commit>
   ```
   If ANY done commit is not pushed, stop immediately and tell the user to push first. Do not proceed with anything else.

4. **Resolve threads on GitHub.** For each section with `status: done`:
   a. Post a comment on the thread:
      ```
      gh api graphql -f query='
        mutation($threadId: ID!, $body: String!) {
          addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $threadId, body: $body}) {
            comment { id }
          }
        }' -f threadId=<thread-id> -f body='resolved by `<commit-hash>`'
      ```
   b. Resolve the thread:
      ```
      gh api graphql -f query='
        mutation($threadId: ID!) {
          resolveReviewThread(input: {threadId: $threadId}) {
            thread { isResolved }
          }
        }' -f threadId=<thread-id>
      ```

5. **Clean up the worktree:**
   ```
   git -C <worktree> worktree remove <worktree-path>
   ```
   Then remove the empty repo directory if it's now empty:
   ```
   rmdir ~/w/{repo} 2>/dev/null || true
   ```

6. **Done.** Tell the user how many threads were resolved and that the worktree is cleaned up.
