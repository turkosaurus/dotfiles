# Style Guilde

## plans
Every worktree under `~/w/<repo>/<branch>/` has two plan files (both gitignored):

- `plan.toml` — structured, tool-managed. Canonical schema: `dotfiles/cmd/work/plan.go`. Fields: `title`, `status`, `due`, `tasks[]`, `slack`, `[[issue]]`, `[pr]` with `[[pr.comment]]`. Populated by `work sync`.
- `plan.md` — freeform scratchpad. Human notes, LLM output, outlining, temp thoughts. The `work` tool never touches this file. Top of the doc is for humans; bottom is for LLMs.

The old `~/w/plan.md` aggregate is retired. The live cross-worktree view is `work list`.

### editing plan.toml

Use `work` verbs for anything that has one — never hand-edit or `sed`:

| Change | Command |
|---|---|
| new worktree/task | `work new [title]` |
| set status | `work set -o\|-w\|-W\|-c` (open, waiting, working, closed) |
| refresh from GitHub | `work sync` |
| open in $EDITOR | `work edit` |
| parse-check | `work validate [-a]` |

For fields without a verb yet — most notably `[[pr.comment]]` entries used by `/pr-review` — use `yq -p toml -o toml -i` so structure and quoting stay sound. Example, append a comment:

```bash
yq -p toml -o toml -i '.pr.comment += [{"title":"…","status":"open","source":"…","author":"…","thread":"…","fix_ref":"","comment":"…","plan":"","reply":""}]' plan.toml
```

Update a comment's status by index:

```bash
yq -p toml -o toml -i '.pr.comment[0].status = "closed"' plan.toml
```

## editing toml (non-plan)
For other TOML files (e.g. `mise.toml`, config files), use `yq -p toml -o toml -i` — never `sed`, `awk`, or hand-editing when doing bulk changes.

## usage rules
- Never commit or push.
- Permissions should allow extensive read, very limited write.
- Add tool permissions to `~/dotfiles/home/.claude/settings.base.json` and run `dotsync -lv` to sync.
- If the same permission needs to be requested repeatedly, write a script with narrow permissions that I can review and approve for the session.
- All config or learned behavior should be exclusively in ~/dotfiles

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

