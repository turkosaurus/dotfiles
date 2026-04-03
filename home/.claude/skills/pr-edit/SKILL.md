---
name: pr-edit
description: Implement fixes from PLAN.md in a PR worktree — edits only, never commits or pushes
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
   c. Update that section's `status` to `done` in PLAN.md.
   d. Write the updated PLAN.md so progress is saved.

3. **Skip sections** with `status: skip` or `status: done`. Do not touch them.

4. **NEVER commit or push.** Do not run `git commit` or `git push`. Only edit files. The user will stage, commit, and push themselves.

5. **Done.** Tell the user:
   - How many items were implemented vs skipped
   - Remind them to review the changes (`git diff`), then commit and push when ready, and run `/pr-update <number>` to resolve the threads on GitHub
