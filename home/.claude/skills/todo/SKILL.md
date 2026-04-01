---
name: todo
description: Morning planning assistant — reviews GitHub project board, PRs, and Slack to recommend what to focus on today
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(gh *), Bash(cat *), Read, Write, mcp__slack__*
argument-hint: <github-project-url>
---

# Daily Planning Assistant

Help the user plan their day by gathering context from GitHub and Slack, then recommending a prioritized list of focus items.

## Input

`$ARGUMENTS` is a GitHub Project URL (e.g. `https://github.com/orgs/acme/projects/7`).

## Steps

1. **Gather context in parallel.** Collect all of the following:

   a. **Project board** — fetch items from the GitHub project, focusing on:
      - Items assigned to the current user (`gh project item-list` or the GraphQL API)
      - Their status (todo, in progress, in review, blocked)
      - Priority labels if present
      ```
      gh project item-list <number> --owner <owner> --format json
      ```

   b. **Review requests** — PRs where the user's review is requested:
      ```
      gh search prs --review-requested=@me --state=open --json number,title,repository,updatedAt,url
      ```

   c. **Authored PRs needing attention** — user's own PRs that have new reviews or comments:
      ```
      gh search prs --author=@me --state=open --json number,title,repository,reviewDecision,url
      ```

   d. **Slack** — use the Slack MCP tools to check:
      - Saved items / "later" list (if the MCP exposes this — try first, fall back gracefully)
      - Recent DMs and mentions
      - Threads the user is in that have new replies
      - Anything that looks like it needs a response or action

   e. **Pomo log** — read `~/.pomo.log` (last 10 lines) to see what was worked on recently. This gives context on momentum and what was left unfinished.

2. **Analyze and prioritize.** Think about what matters most:
   - **Unblock others first**: review requests and PRs waiting on the user
   - **In-progress work**: things already started (especially recent pomo sessions that didn't finish)
   - **Sprint commitments**: items in the current sprint that aren't started yet
   - **Slack**: messages that need responses, especially time-sensitive ones
   - **Backlog**: only if the above are light

3. **Cluster into social and focus blocks.**

   - **Social**: Slack replies, threads, meetings — reactive work. Batch together.
   - **Focus**: PR reviews, coding, bug fixes, feature work — anything requiring concentration. Bundle related technical work (e.g. a sprint item and a backlog bug in the same package make a good single pomo session).

   Recommend social first to unblock others, then protected focus time — unless a deadline says otherwise.

4. **Write `~/today.md`** with this format:

   ```markdown
   # <today's date>

   ## Recommended focus

   1. **<action>** — <why this is urgent/important>
      `pomo "<task name>"`
   2. ...
   3. ...

   ## Review requests
   - [ ] <PR title> — <repo> #<number>

   ## Your PRs
   - [ ] <PR title> — <status: approved/changes requested/waiting>

   ## Slack
   - [ ] <summary of thread or DM needing response>

   ## Sprint items not started
   - [ ] <issue title> — <priority>
   ```

   Each recommended focus item should include a ready-to-paste `pomo` command.

5. **Present the plan.** Show the user the recommended focus section directly in the conversation. Keep it brief — 3-5 items max. Don't overwhelm.

## Notes

- If `~/today.md` already exists for today's date, update it rather than overwriting — the user may have added notes.
- The recommended focus should be opinionated. Don't just list everything. Make a call on what matters most and why.
- Keep Slack summaries vague enough to avoid leaking sensitive content into the file — just enough to remind the user what needs attention.
