---
name: pr-edit
description: Implement fixes from PLAN.md in a PR worktree, one commit per fix
user-invocable: true
disable-model-invocation: true
allowed-tools: Read, Grep, Glob, Edit, Bash(git *), Write
---

# PR Edit

Implement the fixes described in PLAN.md.

## Prerequisites

You must be in a worktree directory that contains a `PLAN.md` created by `/pr-plan`. Check for `PLAN.md` in the current working directory. If it doesn't exist, tell the user and stop.

## Steps

1. **Read `PLAN.md`** from the current directory.

2. **For each `##` section with `status` of `pending` in the metadata table**, in order:
   a. Read the `file` from the metadata table and understand the code.
   b. Implement the fix described in the `### Reply` section.
   c. Stage only the changed files.
   d. Commit with a message like: `fix: <the ## section title>`
   e. Update that section's metadata table in PLAN.md:
      - Set `status` to `done`
      - Set `commit` to the 7-char abbreviated hash
   f. Write the updated PLAN.md after each commit so progress is saved.

3. **Skip sections** with `status: skip` or `status: done`. Do not touch them.

4. **One commit per fix.** Do not bundle multiple thread fixes into one commit.

5. **Do not push.** Do not run `git push`. The user will push when ready.

6. **Done.** Tell the user:
   - How many items were implemented vs skipped
   - Remind them to review the changes (`git log --oneline`), push when ready, then run `/pr-update <number>` to resolve the threads on GitHub
