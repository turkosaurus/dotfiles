# Style Guilde

## plans

Always read `plan.toml` in the cwd as a first step.

Every worktree under `~/w/<repo>/<branch>/` has two plan files (both gitignored):

- `plan.toml` — structured, tool-managed. Canonical schema:
  `dotfiles/cmd/work/plan.go`. Fields: `title`, `status`, `due`,
  `tasks[]`, `slack`, `[[issue]]`, `[[pr]]` with `[[pr.comment]]`.
  Populated by `work sync`.
- `plan.md` — freeform scratchpad. Human notes, LLM output, outlining,
  temp thoughts. The `work` tool never touches this file. Top of the
  doc is for humans; bottom is for LLMs.

Use `work list` for the cross-worktree view.

### what goes where in plan.toml

Two fields look similar but hold different things — keep them straight:

- **`tasks[]`** — the ordered backlog of things **I** need to do to
  resolve what this plan represents. Anything actionable belongs here:
  outgoing review comments I need to author, fixes I need to make,
  follow-ups to open on other PRs, deploys to run, decisions I owe
  someone. Position matters; the top of the list is what's next.
- **`[[pr]].comment`** — incoming review comments on **code I own**
  (my PR) that I need to respond to. Written by `/pr-review` from
  fetched GitHub review comments. Each entry has `status`, `reply`,
  `fix_ref`, etc. to track my response.

**Rule of thumb**: if it's a thing I need to *do*, it goes in
`tasks[]`. If it's a review comment someone left on my code that I
need to reply to or address, it goes in the matching `[[pr]].comment`.
When in doubt (e.g. "author a comment on someone else's PR"), it's a
task — the destination is outbound.

Anything queued for later — even inside a code-review flow — should
land in `tasks[]` so `work list` and `work sync` surface it.

### editing plan.toml

Prefer, in order:

1. **`/work update`** — the skill that owns plan.toml mutations.
   Callable directly (`/work update "..."`) or from another skill.
   Handles routing (worktree tasks vs top-level task vs
   `[[pr.comment]]`) and the multi-line formatting rules.
2. **`work` CLI verbs** for things with a verb:

| Change | Command |
|---|---|
| new worktree/task | `work new [title]` |
| set status | `work status -o\|-w\|-W\|-c` (open, waiting, working, closed) |
| refresh from GitHub | `work sync` |
| open in $EDITOR | `work edit` |
| parse-check | `work validate [-a]` |
| fold task into worktree | `work promote` |

3. **`yq -p toml -o toml -i`** as a last resort for fields without a
   verb — most notably `[[pr.comment]]` entries. `/work update` uses
   this under the hood; call it directly only when the skill layer
   isn't a fit. Never hand-edit or `sed`. `[[pr]]` is an array, so
   index into it explicitly:

```bash
yq -p toml -o toml -i '.pr[0].comment += [{"title":"…","status":"open","source":"…","author":"…","thread":"…","fix_ref":"","comment":"…","plan":"","reply":""}]' plan.toml
```

Update a comment's status by PR index and comment index:

```bash
yq -p toml -o toml -i '.pr[0].comment[0].status = "closed"' plan.toml
```

## editing toml (non-plan)
For other TOML files (e.g. `mise.toml`, config files), use `yq -p toml -o toml -i` — never `sed`, `awk`, or hand-editing when doing bulk changes.

## usage rules
- Never commit or push.
- Permissions should allow extensive read, very limited write.
- Add tool permissions to `~/dotfiles/home/.claude/settings.base.json` and run `dotsync -lv` to sync.
- If the same permission needs to be requested repeatedly, write a script with narrow permissions that I can review and approve for the session.
- All config or learned behavior should be exclusively in ~/dotfiles

## prose & text blocks

- Wrap markdown, comments, docstrings, and skill-instruction prose at
  ~80 characters. Longer lines are hard to diff and hard to read
  side-by-side.
- Prefer one topic per paragraph. Break lists onto their own lines
  rather than packing bullets into a single line.

### TOML multi-line strings

For any TOML string field long enough to warrant a multi-line literal
(`tasks[]`, `[[pr.comment]].plan`, `.reply`, `.comment`, etc.):

- Triple single quotes on their own lines.
- Every content line ≤ 70 characters, hand-wrapped. Never rely on
  terminal soft-wrap — a `cat plan.toml` on a 70-char terminal must
  render every line without breaking.
- First content line ≤ 40 characters. It acts as a dense heading; no
  mandatory blank line after it.

```toml
tasks = [
    '''
    rename x -> count
    Update Store.Add so the loop counter is named
    clearly and matches the rest of the file's
    conventions.
    ''',
]
```

`/work update` enforces this when it writes multi-line fields.

## structured output

Mechanical actions — proposing a fix, adding a task, updating a PR
comment's status — must land in `plan.toml` (or in a `work` task
file), not in chat prose. Chat is a **receipt**, not a re-statement.
`/work update` is the composition surface: every skill that mutates
plan state should route writes through it.

Two receipt formats:

**Human receipt.** Any skill after a mutation shows a compact table
in chat naming the target paths and what landed. No prose describing
the change — the write *is* the description.

```
wrote:
  pr[0].comment[2].plan   ← 42 chars
  pr[0].comment[2].status ← done
  tasks[+]                ← 'follow up on #4412'
```

**Machine receipt.** `/work update` returns `key=value` lines other
skills can parse without regex tricks.

```
target=pr[0].comment[2].plan
action=set
status=ok
```

On ambiguity `/work update` returns `status=ambiguous`, plus `reason=`
and `suggest=` fields. The caller decides whether to prompt or fall
back.

## tool use
Prefer classic unix workflows like piping and writing to files.
Preferred tools in `~/.mise.toml`.

## go

### errors & formatting
- Errors should _always_ be handled idomatically, using wrapping.
- Aim for 80 characters per line, but 100 or 120 can be okay sometimes.
- Keep key/value pairs aligned.
```go
if err != nil {
    slog.Error(ctx, "doing thing", 
        "key1", value1,
        "key2", value2,
    )
    return fmt.Errorf("doing thing: %w", err)
}
```

- Variable length should be inversely proportional to it's scope and life.
- Function names should be just `func Noun()` when returning data (not `func GetNoun()`), using `func NounVerb()` for other cases, or just `func Verb()` for purely functional functions.

## bash
- Always use `#!/usr/bin/env bash`
- Handle errors robustly, the `if !` pattern is nice

