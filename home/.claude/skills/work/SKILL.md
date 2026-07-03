---
name: work
description: Verb-dispatched skill wrapping the `work` CLI. `update` routes intent into plan.toml (callable by other skills); `plan` reviews and prioritizes.
user-invocable: true
disable-model-invocation: false
allowed-tools: Bash(work *), Bash(yq *), Bash(gh *), Bash(gh-sprint-fetch), Bash(cat *), Bash(ls *), mcp__slack__*, Read
argument-hint: <update|plan> [args]
---

# /work

One skill, two verbs. First positional argument dispatches.

- `update` — write to plan.toml. Callable by other skills.
- `plan` — review and prioritize. Read-only.

If no verb is given, fall through to `plan` (the common case: "what's
next in this worktree?"). If the first arg is neither `update` nor
`plan` nor a recognized `plan`-mode focus keyword, treat it as
freeform intent for `update`.

## `/work update`

Route intent into `plan.toml`. Always emit a machine receipt on
stdout so callers can parse the result without regex.

### Input

- Positional: `<intent>` — a natural-language description of the
  thing to record. Required.
- `--to <target>` — optional hint. One of:
  - `tasks` — append to current worktree's `plan.toml` `tasks[]`.
  - `new-task` — create a top-level task via `work new "<title>"`.
  - `pr-comment=<thread-id>` — write to a `[[pr.comment]]` entry
    matching the given thread ID.
- `--status <open|waiting|working|closed|done>` — set status on the
  target (mostly for pr-comment updates).
- `--fix-ref <ref>` — set `fix_ref` on a pr-comment target.
- `--reply <text>` — set `reply` on a pr-comment target.

### Routing (freeform mode, no `--to`)

Decide from context:

| Signal | Target |
|---|---|
| Standing in a worktree, intent is local ("fix X here", "add tests") | append to that worktree's `tasks[]` |
| Not tied to any worktree | `work new "<title>"` — top-level task |
| Intent references a PR/issue already in some worktree's plan | that worktree's `tasks[]` |
| Explicit review-comment thread ID present | matching `[[pr.comment]]` |

When ambiguous, return `status=ambiguous` rather than guessing.

### Write mechanism

Prefer in order:

1. `work` verbs: `work new "<title>"` for top-level task creation.
2. `yq -p toml -o toml -i` for fields without a verb. Path shape:
   `.pr[<n>].comment[<m>].{status,plan,reply,fix_ref}` or
   `.tasks += ['''...''']`.
3. Never hand-edit or `sed`.

### Multi-line formatting

Values longer than ~40 chars **must** be written as TOML triple-
single-quoted literals, hand-wrapped per AGENTS.md rules:

- Every content line ≤ 70 chars.
- First content line ≤ 40 chars (dense heading).
- Triple quotes on their own lines.

Because `yq`'s TOML encoder emits double-quoted strings with `\n`
escapes, for multi-line values first write via `strenv`, then run
`work edit` (or note in the receipt) that the field should be
reformatted to triple-quoted by hand on next visit. Both forms parse
identically.

```bash
PLAN=$(cat <<'EOF'
rename x -> count
Update Store.Add so the loop counter is named
clearly and matches the rest of the file.
EOF
) yq -p toml -o toml -i '.pr[0].comment[2].plan = strenv(PLAN)' plan.toml
```

### Machine receipt

Print to stdout, one `key=value` per line, no quoting:

```
target=pr[0].comment[2].plan
action=set
status=ok
```

On ambiguity:

```
status=ambiguous
reason=intent mentions PR #4412 but current worktree has no matching pr
suggest=/work update "..." --to new-task
```

### Human receipt (for the calling skill or the user)

The caller re-emits the machine receipt as a compact table:

```
wrote:
  pr[0].comment[2].plan   ← 42 chars
  pr[0].comment[2].status ← done
  tasks[+]                ← 'follow up on #4412'
```

Never write prose describing the change. The write is the
description.

### Examples

Freeform, in a worktree:

```
/work update "fix rustfmt in src/lib.rs"
```

Freeform, ambiguous → returns `status=ambiguous`:

```
/work update "look at whatever Bob is asking about"
```

Hinted, from `/pr-review`:

```
/work update "add nil-check to Fetch caller" --to pr-comment=T_ABC1
/work update --to pr-comment=T_ABC1 --status done --fix-ref abc1234
```

## `/work plan`

Review and prioritize. Never writes. Two modes:

- **Focused** (no arg, inside a worktree; or `/work plan <focus>`
  where `<focus>` names a branch or repo): read that worktree's
  `plan.toml`, consult ordered `tasks[]`, recommend the top item and
  flag dependencies. Cross-reference `[pr]`, `[[issue]]`, `[slack]`.
- **Daily** (`/work plan --daily`, or invoked outside any worktree
  with no arg): full sweep.

### Daily sweep

1. **Refresh inventory** — sync every worktree's plan from GitHub:
   ```bash
   work sync -a --yes
   work list
   ```
   Surface any `work validate -a` errors first — broken plan files
   need attention before prioritization is meaningful.

2. **Slack context** — via `mcp__slack__*`:
   - saved items / bookmarks
   - recent DMs and @-mentions
   - threads with new replies for me
   Fall back gracefully if the MCP is unavailable.

3. **Sprint context** — `gh-sprint-fetch` for active sprint name,
   dates, days remaining. If the invocation includes a GitHub project
   URL, run:
   ```bash
   gh project item-list <number> --owner <owner> --format json
   ```
   and filter for items assigned to me that aren't Done.

4. **Analyze** — bucket every item into **today** or **deprioritized**
   using this order:
   1. Merge-ready PRs (approved, CI green) — ship immediately.
   2. Unblock others — review requests, especially stale ones.
   3. In-progress sprint work.
   4. Not-started sprint commitments (urgent if sprint ends this
      week).
   5. Backlog — deprioritize unless it overlaps a today item.

5. **Present** — cluster **social first** (short reactive work), then
   **focus** (protected long blocks). One item per line with a link.

```markdown
# YYYY-MM-DD

**Sprint:** <name> — ends <date> (<N> days remaining)

## Social (batch)
- [ ] Review: [<PR title>](<url>) — <repo>, <context>
- [ ] Slack: <vague summary>

## Focus (protected)
- [ ] <repo:branch>: <specific next step>
  - [ ] [<PR title>](<url>) — <specific next step>

## Deprioritized
- [ ] <item> — <reason: waiting, blocked, low priority>
```

Be specific about next steps ("fix rustfmt in src/lib.rs",
"rebase on main after #2551 merges", "address review comment on
retry logic") — never vague like "continue work" or "finish PR".

Keep Slack summaries vague enough to avoid leaking sensitive content.

### Capturing new work

When the plan surfaces work that isn't tracked anywhere (a Slack ask,
a sprint item without a worktree), don't write it — suggest a
`/work update` invocation instead:

```
Suggested: /work update "look at X for so-and-so"
```

## Notes

- **Never** write `plan.md`. That's the freeform scratchpad; `/work`
  never touches it.
- The `plan.toml` at each worktree is the structured, tool-managed
  metadata. `tasks[]` ordering is the user's curated intent —
  respect it.
- Be opinionated about what belongs on today's list. Make a call.
  Everything gets listed — nothing silently dropped.
- Never commit or push.
