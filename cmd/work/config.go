package main

// TODO: config file at ~/w/.config.toml for project-specific stuff.
// Fields once we need them:
//   - github.project_url    (sprint board etc.)
//   - github.org            (default owner for un-scoped queries)
//   - slack.workspace       (workspace mapping if we ever want it)
//   - github issue repos
//
// Config lives OUTSIDE dotfiles because it holds work-specific org details.
// ~/w/ is naturally per-machine and not synced to the public dotfiles repo.
//
// Not implementing now — nothing in the current tool needs it. Placeholder so
// the file layout is obvious when the need arises.
