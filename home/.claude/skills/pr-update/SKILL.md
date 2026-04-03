---
name: pr-update
description: Implement PR plan fixes one at a time, prompt for acceptance, resolve threads on GitHub, then clean up
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh *), Bash(git *), Read, Grep, Glob, Edit, Write, AskUserQuestion
argument-hint: <pr-number>
---

# PR Update

Implement fixes from a PR plan, get user approval, then resolve threads on GitHub.

## Input

`$ARGUMENTS` is a PR number.

## Prerequisites

Look for `PR$ARGUMENTS.md` at the git toplevel. If it doesn't exist, tell the user to run `/pr-plan $ARGUMENTS` first and stop.

## Steps

1. **Read `PR$ARGUMENTS.md`.**

2. **For each `##` section in order**, check `status` in the metadata table:
   - `skip` or `done` — skip it silently.
   - `pending` — implement it:
     a. Read the `file` from the metadata table and understand the surrounding code.
     b. Implement the fix described in the `### Reply` section. **Only edit files — never commit or push.**
     c. Show the user what changed (mention the file and the change).
     d. Ask the user to accept or reject. If rejected, revert the change and move on.
     e. If accepted, update `status` to `done` in `PR$ARGUMENTS.md`.

3. **After all sections are processed**, if any items were accepted:
   a. Tell the user to review (`git diff`), commit, and push.
   b. **Stop here and wait.** Do not proceed until the user confirms they have pushed.

4. **Resolve threads on GitHub.** For each `done` section, read the `thread` value:
   a. Post a reply on the thread:
      ```
      gh api graphql -f query='
        mutation($threadId: ID!, $body: String!) {
          addPullRequestReviewThreadReply(input: {pullRequestReviewThreadId: $threadId, body: $body}) {
            comment { id }
          }
        }' -f threadId=<thread-id> -f body='Addressed.'
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

5. **Clean up.** Delete `PR$ARGUMENTS.md`:
   ```
   git rm PR$ARGUMENTS.md 2>/dev/null || rm PR$ARGUMENTS.md
   ```

6. **Done.** Tell the user how many threads were resolved vs skipped.
