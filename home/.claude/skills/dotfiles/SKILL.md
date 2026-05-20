---
name: dotfiles
description: Make durable changes to the user's dotfiles repo (~/dotfiles) — permissions, hooks, env vars, scripts, skills, agents. Use when the user asks to add/change a permission, install a hook, add a tool to allow-list, write a new bin script, or otherwise wants a change to persist across machines via dotsync. Knows the symlink layout, where each kind of config lives, and the dotsync workflow.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read, Edit, Write, Bash(dotsync -lv), Bash(ls:*), Bash(find:*), Bash(jq:*), Bash(git status:*), Bash(git diff:*), Bash(git log:*)
argument-hint: <what to change>
---

# Dotfiles Editor

Apply durable, machine-portable configuration changes to `~/dotfiles`. The dotfiles repo is the single source of truth — anything edited under `$HOME` directly is ephemeral and will be overwritten by the next `dotsync`.

## Hard rules

- **Never commit or push.** Stage changes; the user reviews and commits.
- **Always edit files under `~/dotfiles/home/`, never the symlinks under `$HOME`.** The targets in `$HOME` are symlinks back to `~/dotfiles/home/...` — editing the symlink works, but is confusing. Edit the source directly.
- **`settings.base.json` is the durable file.** `settings.json` is generated/mirrored from it. Edit `settings.base.json`.
- **Run `dotsync -lv` after every change** so symlinks are refreshed locally without pulling remote.
- Prefer minimal, targeted edits. Don't restyle JSON or reorder keys.
- **Never put secrets in dotfiles.** `~/.secrets` is *not* tracked — it lives only on the local machine and is sourced by `~/.zshrc` and `~/.bashrc` (see `[[ -f ~/.secrets ]] && source ~/.secrets`). API keys, tokens, and credentials go there as `export FOO=...`. Don't read it, don't print its contents, and never copy values from it into a tracked file.

## Repo layout

```
~/dotfiles/
├── home/                 # everything here is symlinked into $HOME by dotsync
│   ├── .claude/
│   │   ├── CLAUDE.md             # global user instructions
│   │   ├── settings.base.json    # DURABLE — edit this for permissions/hooks/env
│   │   ├── settings.json         # mirror — usually identical to base
│   │   └── skills/<name>/SKILL.md
│   ├── bin/                      # scripts here are on $PATH
│   └── ...                       # other dotfiles (.zshrc, .mise.toml, etc.)
├── init.sh                       # first-run installer
└── home/bin/dotsync              # the sync script itself
```

`dotsync` walks `~/dotfiles/home/` and symlinks every file into the matching `$HOME` path. Directories are `mkdir -p`'d, files become `ln -sf` symlinks.

Flags:
- `dotsync` — pull remote, then symlink
- `dotsync -l` — local only (skip git pull); use this after local edits
- `dotsync -v` — verbose
- `dotsync -lv` — the standard "I just edited something" invocation

## Where each change goes

| User asks for…                                | File to edit                                              |
|-----------------------------------------------|-----------------------------------------------------------|
| Allow a Bash/MCP/etc tool                     | `home/.claude/settings.base.json` → `permissions.allow`   |
| Deny reading a path                           | `home/.claude/settings.base.json` → `permissions.deny`    |
| Env var for Claude Code                       | `home/.claude/settings.base.json` → `env`                 |
| Hook (Notification/TaskCompleted/PreToolUse…) | `home/.claude/settings.base.json` → `hooks`               |
| New CLI script                                | `home/bin/<name>` (chmod +x, `#!/usr/bin/env bash`)       |
| New skill                                     | `home/.claude/skills/<name>/SKILL.md`                     |
| New global instruction / style rule           | `home/.claude/CLAUDE.md`                                  |
| Shell config                                  | `home/.zshrc` (or appropriate dotfile)                    |
| Tool versions                                 | `home/.mise.toml`                                         |
| Secret / API token / credential               | `~/.secrets` (untracked, sourced by `.zshrc`/`.bashrc`)   |

If the user repeatedly approves the same broad permission, prefer writing a narrow script in `home/bin/` and allow-listing just that script (e.g., `Bash(my-script:*)`).

## Permission entry style

Match the existing style in `settings.base.json`:
- One entry per line, alphabetical-ish within its group
- Glob suffix: `Bash(gh pr view:*)` not `Bash(gh pr view *)`
- Don't add `Read(**)` style broad rules — they're already there
- Never widen `deny`-listed paths

## Workflow

1. **Read** `settings.base.json` (or the relevant file) first to see current state.
2. **Edit** the file under `~/dotfiles/home/...` with a minimal, targeted change.
3. **Validate** JSON files with `jq . <file> > /dev/null` if you touched JSON.
4. **Run `dotsync -lv`** so the symlinks refresh locally.
5. **Show the diff** (`git -C ~/dotfiles diff`) and stop. Do not commit.

## New script checklist

When adding a script to `home/bin/`:
- `#!/usr/bin/env bash` shebang
- Robust error handling (`set -euo pipefail` or `if ! cmd; then ...`)
- `chmod +x` the file
- If it needs Claude Code permissions, allow-list it in `settings.base.json` with `Bash(script-name:*)`
- Run `dotsync -lv` so it becomes available on `$PATH`

## New skill checklist

When adding a skill to `home/.claude/skills/<name>/`:
- Create `<name>/SKILL.md` with frontmatter: `name`, `description`, optionally `user-invocable: true`, `disable-model-invocation: true`, `allowed-tools`, `argument-hint`
- Description is what the model sees to decide relevance — make it specific
- Run `dotsync -lv` to symlink it into `~/.claude/skills/`

## What NOT to do

- Don't edit `~/.claude/settings.json` directly (it's the symlink target — confusing and won't survive a fresh clone).
- Don't `git commit` or `git push` in `~/dotfiles`. Stop after staging/showing the diff.
- Don't add broad permissions like `Bash(*)` or `Bash(gh:*)` — prefer narrow patterns or a wrapper script.
- Don't create a new file when an existing one already covers the concern.
