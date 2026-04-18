---
name: plan-me
description: Morning planning assistant — reviews GitHub project board, PRs, and Slack to recommend what to focus on today
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(todo-fetch), Bash(gh-sprint-fetch), Bash(gh project item-list *), Bash(gh api graphql *), Read, Write, mcp__slack__*
argument-hint: <github-project-url>
---

# Daily Planning Assistant

Help the user plan their day by gathering context from GitHub and Slack, then recommending a prioritized list of focus items.

## Input

`$ARGUMENTS` is an optional GitHub Project URL (e.g. `https://github.com/orgs/acme/projects/7`).

## Steps

1. **Fetch data.** Run `todo-fetch` and `gh-sprint-fetch` and capture their
   stdout as NDJSON — one `{"type":"<name>","data":<json>}` line per source:

   From `todo-fetch`:
   - `review-requests` — PRs where your review is requested
   - `my-prs` — your open PRs (with isDraft)
   - `pr-statuses` — reviewDecision + CI status per PR
   - `issues` — issues assigned to you
   - `pomo` — last 10 pomo sessions (string)

   From `gh-sprint-fetch`:
   - `sprint` — one line per active sprint: `project`, `sprint`, `start`,
     `end`, `days_remaining`

   Parse line by line. Skip any line that is not valid JSON (interrupted
   write); note which types are missing and proceed with what you have.

   Use the `sprint` data for the "Sprint ends:" line and urgency framing.
   Do not guess sprint dates.

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

3. **Analyze and categorize.** List everything from the fetched data.
   Categorize each item as either today's plan or deprioritized.

   **Priority order** (for deciding what's today vs deprioritized):
   1. **Merge-ready PRs** — approved, CI green → ship immediately
   2. **Unblock others** — review requests, especially stale ones
   3. **In-progress sprint work** — items already started
   4. **Not-started sprint commitments** — urgent if sprint ends soon
   5. **Backlog** — deprioritize unless it overlaps with work already on today's list

   **Sprint awareness:**
   - Note where the sprint is in its cycle (beginning, mid, end).
   - Flag uncommitted sprint items as urgent if the sprint ends this week.

4. **Cluster into social and focus blocks.**

   - **Social**: Slack replies, reviews — reactive, batch together.
   - **Focus**: coding, bug fixes, feature work — bundle related work.

   Recommend social first to unblock others, then protected focus time.

5. **Write `~/w/plan.md`** using the format below. List ALL items — issues,
   PRs, reviews, slack. Items below the `---` are deprioritized for today
   but still tracked. Every item is a single line. Link everything.

   ```markdown
   # <today's date>

   Sprint ends: <date or "this week" etc.>

   ## Social

   - [ ] [Review: <PR title> #<number>](<pr url>) — <repo>, <context>
   - [ ] <Slack summary needing response>

   ## Focus

   - [ ] [<issue title>](<issue url>)
     - [ ] [<PR title> #<number>](<pr url>) — <specific next step>
   - [ ] [<issue title>](<issue url>)
     - [ ] [<PR title> #<number>](<pr url>) — <specific next step>

   ---

   ## Deprioritized

   - [ ] [<issue title>](<issue url>) — <reason: waiting, blocked, low priority, etc.>
   - [ ] [<PR title> #<number>](<pr url>) — <reason>
   ```

   Be specific in next steps (e.g. "fix rustfmt in src/lib.rs",
   "rebase on main after #2551 merges", "address review comment on
   retry logic") — not vague like "continue work" or "finish PR".

6. **Present the plan.** Show the focus section in the conversation. Keep it brief.

## Notes

- If `~/w/plan.md` already exists for today's date, update it rather than overwriting.
- Be opinionated about what's today vs deprioritized. Make a call.
- Keep Slack summaries vague enough to avoid leaking sensitive content.
- Everything gets listed — nothing is silently dropped.
