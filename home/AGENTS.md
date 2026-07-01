# Style Guilde

## plans
Every worktree under `~/w/<repo>/<branch>/` has two plan files (both gitignored):

- `plan.toml` — structured, tool-managed. Read/written by the `work` CLI: title, status, due, tasks, slack, issue(s), pr. Populated by `work sync`. Don't hand-edit unless you know what you're doing; use `work` verbs.
- `plan.md` — freeform scratchpad. Human notes, LLM output, outlining, temp thoughts. The `work` tool never touches this file. Top of the doc is for humans; bottom is for LLMs.

The old `~/w/plan.md` aggregate is retired. The live cross-worktree view is `work list`.

## editing toml
Always edit TOML files with `yq -p toml -o toml -i` — never `sed`, `awk`, or
hand-editing when doing bulk changes. yq round-trips the parse so structure
stays sound and quoting/escaping is correct.

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

