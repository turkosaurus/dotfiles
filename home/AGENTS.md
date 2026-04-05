# Style Guilde

## plans
The most important file is a special `plan.md` which will be gitignored, but will contain anything useful about the plan or progress. The top of this document is for humans, the bottom of the document may be used for LLMs planning.

## usage rules
- Never commit or push.
- Permissions should allow extensive read, very limited write.
- If the same permission needs to be requested repeatedly, write a script with narrow permissions that I can review and approve for the session.
- All config or learned behavior should be exclusively in ~/dotfiles

## tool use
Prefer the following tools and classic unix workflows like piping and writing to files.
- git
- diff
- rg (fallback: grep)
- fzf
- jq
- yq
- sq
- sed
- awk
- nvim

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

