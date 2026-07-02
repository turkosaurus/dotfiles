---
name: planner
description: Daily planning — refresh work inventory from GitHub, consult Slack, and recommend today's focus
user-invocable: true
disable-model-invocation: true
allowed-tools: Bash(work *), Bash(todo-fetch), Bash(gh-sprint-fetch), Bash(gh *), Bash(cat *), Bash(ls *), mcp__slack__*, Read
argument-hint: [focus keyword or github project URL]
---

# Planner

Two modes depending on how the user invokes you:

- **Focused** — user says things like "next task here", "what's next", or references a specific worktree/branch. Go straight to section **1a**: read the current or referenced worktree's `plan.toml` and recommend from its ordered `tasks[]`.
- **Full daily plan** — user invokes with no argument, or asks for "today's plan". Run the full sweep (sections 1 → 5).

The live inventory is `work list`; per-worktree scratchpads are `<worktree>/plan.md`.

## 1. Refresh inventory

Sync every worktree's `plan.toml` from GitHub first (parallel, ~2s for ~30 worktrees):

```bash
work sync -a --yes
```

Then read the current state:

```bash
work list
```

Each row is either a **worktree** (git branch under `~/w/<repo>/<branch>/`) or a **task** (a local file at `~/w/t/{open,waiting,working,closed}/N.toml`). Worktrees carry PR status, unresolved review comments, and closing-issue refs when a plan.toml is populated. Tasks are local-only.

Statuses (both worktrees and tasks share): **open** / **waiting** / **working** / **closed**.

If a row shows the broken-status glyph or `work validate -a` reports errors, surface those first — the file didn't parse and the user needs to know.

## 1a. Per-project ordered tasks (the primary guidance signal)

When the user asks about a specific worktree or is standing in one, read its
`plan.toml` and consult the **`tasks` array** — it's an **ordered backlog**
the user has curated. Position matters; treat item 1 as "do this next" unless
the user says otherwise.

```bash
# From inside a worktree, or with an explicit path:
cat plan.toml
# The `tasks = [ ... ]` array. Each string (or triple-quoted block) is one
# task, in the order the user wants to tackle them.
```

Cross-reference `tasks` against `[pr]`, `[[issue]]`, and `[slack]` for context:
if a task references a PR/issue that's already in the plan, note the linkage.
Task entries can be brief prose or freeform notes — parse for what they mean,
don't rely on rigid syntax.

When recommending "what's next," lean on this ordered list. Recommend the top
few items, and if any depend on config or unimplemented tooling (e.g., a task
that says "needs config file first"), flag the dependency.

## 2. Slack context

Use `mcp__slack__*` tools:

- Saved items / bookmarks
- Recent DMs and @-mentions
- Threads with new replies that need my attention

Fall back gracefully if Slack MCP is unavailable — note it and proceed.

## 3. Sprint context

Run `gh-sprint-fetch` for the active sprint's name, dates, and days remaining. Use this to frame urgency ("sprint ends in 2 days" vs "just started").

If `$ARGUMENTS` is a GitHub project URL, also run:

```bash
gh project item-list <number> --owner <owner> --format json
```

and filter for items assigned to me that aren't Done.

## 4. Analyze

For each item across sources, place it in one of two buckets — **today** or **deprioritized** — using this priority order:

1. **Merge-ready PRs** (approved, CI green) — ship immediately
2. **Unblock others** — review requests, especially stale ones
3. **In-progress sprint work** — items already started
4. **Not-started sprint commitments** — urgent if the sprint ends this week
5. **Backlog** — deprioritize unless it overlaps with a today item

Sprint awareness:
- Note where the sprint is (beginning, mid, end).
- Flag uncommitted sprint items as urgent when the sprint ends this week.

## 5. Present

Show the plan in conversation. Cluster social first (short reactive work), then focus (protected long blocks). Every item on its own line with a link. Format:

```markdown
# YYYY-MM-DD

**Sprint:** <name> — ends <date> (<N> days remaining)

## Social (batch)
- [ ] Review: [<PR title>](<url>) — <repo>, <context>
- [ ] Slack: <vague summary of what needs a response>

## Focus (protected)
- [ ] <repo:branch>: <specific next step>
  - [ ] [<PR title>](<url>) — <specific next step>
- [ ] <repo:branch>: <specific next step>

## Deprioritized
- [ ] <item> — <reason: waiting, blocked, low priority>
```

Be specific about next steps ("fix rustfmt in src/lib.rs", "rebase on main after #2551 merges", "address review comment on retry logic") — never vague like "continue work" or "finish PR".

Keep Slack summaries vague enough to avoid leaking sensitive content.

## 6. Chores (optional)

If the plan surfaces new work that isn't tied to a worktree or PR (e.g., a Slack ask "can you look at X" or a task from the sprint board that hasn't started), offer to create it as a chore:

```bash
work chore "look at X for so-and-so"
```

Chores show up in `work list` alongside worktrees and are the durable place for "things to do that don't have code yet".

## Notes

- If the user wants to keep freeform notes for a worktree, `<worktree>/plan.md` is available as a scratchpad. Don't touch it unless asked.
- The `plan.toml` at each worktree is the structured, tool-managed metadata. The `tasks[]` array is the ordered backlog — respect its ordering as the user's intent. Prefer `work` verbs for edits; fall back to `yq -p toml -o toml -i` only for fields without a verb (see `~/AGENTS.md`).
- Be opinionated about what belongs on today's list. Make a call.
- Everything gets listed — nothing silently dropped.
