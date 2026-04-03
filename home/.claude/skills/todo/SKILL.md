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

2. **Analyze and prioritize.** This is a plan for ONE day, not a mirror of the project board. Be opinionated.

   **Priority order:**
   1. **Merge-ready PRs** — approved, CI green → ship immediately
   2. **Unblock others** — review requests, especially fresh ones
   3. **In-progress sprint work** — items already started, especially recent pomo sessions
   4. **Not-started sprint commitments** — be aware of where the sprint is in its iteration. If the sprint ends soon, these become urgent.
   5. **Drive-by backlog opportunities only** — a backlog item is relevant ONLY if it overlaps with sprint/PR work already planned (e.g. same package, same service, two problems one PR). Never list backlog items on their own.

   **Backlog rules:**
   - Do NOT list the user's full backlog. That's what the project board is for.
   - Backlog items only surface when they can piggyback on other work, or when all sprint items are complete.

   **Sprint awareness:**
   - Note where the sprint is in its cycle (beginning, mid, end of week).
   - If the sprint ends this week, flag uncommitted sprint items as urgent.
   - If the sprint just started, there's more room to be strategic.

3. **Cluster into social and focus blocks.**

   - **Social**: Slack replies, threads, meetings — reactive work. Batch together.
   - **Focus**: PR reviews, coding, bug fixes, feature work — anything requiring concentration. Bundle related technical work (e.g. a sprint item and a backlog bug in the same package make a good single pomo session).

   Recommend social first to unblock others, then protected focus time — unless a deadline says otherwise.

4. **Write `~/today.md`** with this format:

   ```markdown
   # <today's date>

   Sprint ends: <date or "this week" etc.>

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
   Keep the total list to what fits in a realistic day (3-5 focus items).

5. **Present the plan.** Show the user the recommended focus section directly in the conversation. Keep it brief — 3-5 items max. Don't overwhelm.

## Notes

- If `~/today.md` already exists for today's date, update it rather than overwriting — the user may have added notes.
- The recommended focus should be opinionated. Don't just list everything. Make a call on what matters most and why.
- Keep Slack summaries vague enough to avoid leaking sensitive content into the file — just enough to remind the user what needs attention.
- The output is a daily plan, NOT a project board dump. If it looks like a list of everything assigned to the user, it's wrong.
