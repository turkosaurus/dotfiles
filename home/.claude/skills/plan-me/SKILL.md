---
name: plan-me
description: Morning planning assistant — reviews GitHub project board, PRs, and Slack to recommend what to focus on today
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(todo-fetch), Bash(gh project item-list *), Read, Write, mcp__slack__*
argument-hint: <github-project-url>
---

# Daily Planning Assistant

Help the user plan their day by gathering context from GitHub and Slack, then recommending a prioritized list of focus items.

## Input

`$ARGUMENTS` is an optional GitHub Project URL (e.g. `https://github.com/orgs/acme/projects/7`).

## Steps

1. **Fetch data.** Run `todo-fetch` and capture its stdout as NDJSON — one
   `{"type":"<name>","data":<json>}` line per source:

   - `review-requests` — PRs where your review is requested
   - `my-prs` — your open PRs (with isDraft)
   - `pr-statuses` — reviewDecision + CI status per PR
   - `issues` — issues assigned to you
   - `pomo` — last 10 pomo sessions (string)

   Parse line by line. Skip any line that is not valid JSON (interrupted
   write); note which types are missing and proceed with what you have.

   If `$ARGUMENTS` is a GitHub project URL, also run:
   ```
   gh project item-list <number> --owner <owner> --format json
   ```
   and filter for items assigned to you that are not Done.

2. **Check Slack** using `mcp__slack__*` tools. Try:
   - Saved items / bookmarks
   - Recent DMs and @-mentions
   - Threads with new replies
   - Anything needing a response

   Fall back gracefully if Slack MCP is unavailable.

3. **Analyze the fetched data** and prioritize. This is a plan for ONE day, not a mirror of the project board. Be opinionated.

   **Priority order:**
   1. **Merge-ready PRs** — approved, CI green → ship immediately
   2. **Unblock others** — review requests, especially stale ones
   3. **In-progress sprint work** — items already started, especially recent pomo sessions
   4. **Not-started sprint commitments** — urgent if sprint ends soon
   5. **Drive-by backlog only** — surface a backlog item ONLY if it overlaps with sprint/PR work already on the list (same package, same service). Never list backlog items alone.

   **Sprint awareness:**
   - Note where the sprint is in its cycle (beginning, mid, end).
   - Flag uncommitted sprint items as urgent if the sprint ends this week.

4. **Cluster into social and focus blocks.**

   - **Social**: Slack replies, reviews — reactive, batch together.
   - **Focus**: coding, bug fixes, feature work — bundle related technical work into pomo sessions.

   Recommend social first to unblock others, then protected focus time.

5. **Write `~/plan.md`** with this format:

   ```markdown
   # <today's date>

   Sprint ends: <date or "this week" etc.>

   ## Recommended focus

   - [ ] **<action>** — <why urgent/important>
         `pomo "<task name>"`
   - [ ] ...

   ## Review requests
   - [ ] <PR title> — <repo> #<number>

   ## Your PRs
   - [ ] <PR title> — <status: approved/changes requested/waiting>

   ## Slack
   - [ ] <summary of thread or DM needing response>

   ## Sprint items not started
   - [ ] <issue title> — <label>
   ```

   Each recommended focus item must include a ready-to-paste `pomo` command.
   Keep recommended focus to 3–5 items max.

6. **Present the plan.** Show the recommended focus section in the conversation. Keep it brief.

## Notes

- If `~/plan.md` already exists for today's date, update it rather than overwriting.
- Be opinionated. Don't just list everything. Make a call on what matters most.
- Keep Slack summaries vague enough to avoid leaking sensitive content — just enough to remind the user what needs attention.
- The output is a daily plan, NOT a project board dump.
