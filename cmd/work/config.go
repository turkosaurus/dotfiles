package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pterm/pterm"
)

// defaultConfigTemplate is the seeded content for a fresh config.toml. TOML
// supports # line comments, so each field carries an inline explanation for
// users editing via `work config`. Keep formatting simple — go-toml won't
// preserve these comments if the file is ever rewritten via saveConfig, so
// treat them as install-time guidance, not durable metadata.
const defaultConfigTemplate = `# work config — per-user settings. Edited with 'work config'.
# Location precedence:
#   $WORK_CONFIG → $XDG_CONFIG_HOME/work/config.toml → ~/.config/work/config.toml

[path]
# Module directory (contains go.mod) that 'work install' rebuilds from.
# Populated automatically the first time you run 'work install'.
source = ""
# Where worktrees live (created by 'work new .'). Default: ~/w.
worktrees = "~/w"
# Where tasks live. Default: <worktrees>/t.
tasks = "~/w/t"
# Where your clones live (informational, for now). Default: ~/p.
repos = "~/p"

[task]
# Default due-date for 'work new' when -d isn't given. Accepts the same
# forms as -d: 2h, 3d, tomorrow, today, or YYYY-MM-DD [HH:MM].
default_due = "tomorrow"

[sprint]
# GitHub project board to pull tasks from on every 'work sync'.
# Leave empty to disable sprint sync.
project_url = ""

# Map local status -> list of GitHub project column names to pull.
# Items in columns not listed here are ignored (nothing created or updated).
# Example:
#   status_fields = { open = ["In Sprint"], working = ["In Progress"], waiting = ["In Review"] }
status_fields = {}

# Filter project items by assignee. "@me" resolves to the authenticated
# GitHub user at sync time; other entries are literal GitHub logins.
# Empty list means "no filter" — every item on the board is synced.
assignees = ["@me"]
`

// runConfig opens the config file in $EDITOR, seeding it with the default
// template if it doesn't exist yet.
func runConfig(_ *configCmd) error {
	p := configPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := os.MkdirAll(path.Dir(p), 0o755); err != nil {
			return fmt.Errorf("config: mkdir %s: %w", path.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(defaultConfigTemplate), 0o644); err != nil {
			return fmt.Errorf("config: write default %s: %w", p, err)
		}
		pterm.Info.Printfln("seeded %s", p)
	} else if err != nil {
		return fmt.Errorf("config: stat %s: %w", p, err)
	}
	if err := openInEditor(p); err != nil {
		return err
	}
	if _, err := loadConfig(); err != nil {
		pterm.Error.Printfln("%v", err)
		return errPrinted
	}
	return nil
}

// config is the per-user config for the work tool. Lives outside dotfiles at
// $XDG_CONFIG_HOME/work/config.toml so org-specific details (project URLs,
// paths, mappings) aren't checked in.
type config struct {
	Path   pathConfig   `toml:"path"`
	Task   taskConfig   `toml:"task"`
	Sprint sprintConfig `toml:"sprint"`

	// LegacySource carries the old [source] section so we can migrate
	// configs written before the rename. loadConfig lifts LegacySource.Path
	// into Path.Source if the latter is unset.
	LegacySource legacySourceConfig `toml:"source"`
}

// taskConfig holds task-creation defaults consulted when the equivalent
// CLI flag isn't provided.
type taskConfig struct {
	// DefaultDue is the fallback for `work new`'s -d flag. Same
	// forms as -d (2h, 3d, tomorrow, YYYY-MM-DD [HH:MM]). Empty →
	// "tomorrow" at midnight (the built-in default).
	DefaultDue string `toml:"default_due"`
}

// pathConfig collects the filesystem knobs. Empty fields fall back to
// baked-in defaults resolved by applyPathConfig. Values may start with
// `~/` — expandTilde replaces it with $HOME at load time.
type pathConfig struct {
	// Source is the module directory (containing go.mod) used by
	// `work install` to rebuild the binary. Set on first install.
	Source string `toml:"source"`
	// Worktrees is where `work new .` moves branches into.
	// Default: ~/w.
	Worktrees string `toml:"worktrees"`
	// Tasks is where per-status task files live. Default:
	// <worktrees>/t.
	Tasks string `toml:"tasks"`
	// Repos is where the user's clones live. Purely informational
	// today — reserved for future use (e.g. auto-discovery of repos
	// to `work new .` from). Default: ~/p.
	Repos string `toml:"repos"`
}

// legacySourceConfig matches the pre-rename `[source]` section. See
// config.LegacySource.
type legacySourceConfig struct {
	Path string `toml:"path"`
}

// sprintConfig holds the GitHub project URL and the mapping from local
// statusKind → list of project column names to pull. Items whose column
// isn't listed anywhere in status_fields are ignored (nothing created,
// nothing updated). Missing project_url disables sprint sync entirely.
//
// Example:
//
//	[sprint]
//	project_url = "https://github.com/orgs/acme/projects/127"
//	status_fields = { open = ["In Sprint"], working = ["In Progress"], waiting = ["In Review"] }
type sprintConfig struct {
	ProjectURL   string              `toml:"project_url"`
	StatusFields map[string][]string `toml:"status_fields"`
	// Assignees filters project items — only items whose assignees include one
	// of these handles are synced. "@me" resolves to the authenticated GitHub
	// user at sync time; anything else is a literal GitHub login. Empty means
	// "no filter" (sync every item).
	Assignees []string `toml:"assignees"`
}

// configPath returns the resolved on-disk location of the config file.
// Precedence: $WORK_CONFIG (if set) → $XDG_CONFIG_HOME/work/config.toml
// → $HOME/.config/work/config.toml. WORK_CONFIG is the escape hatch for
// picking a non-XDG location without touching the surrounding schema.
func configPath() string {
	if p := os.Getenv("WORK_CONFIG"); p != "" {
		return p
	}
	return path.Join(xdgConfigDir("work"), "config.toml")
}

// xdgConfigDir returns $XDG_CONFIG_HOME/<subdir>, falling back to
// $HOME/.config/<subdir>. Never returns an empty path.
func xdgConfigDir(subdir string) string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = path.Join(os.Getenv("HOME"), ".config")
	}
	return path.Join(xdg, subdir)
}

// loadConfig reads the config file. Missing file returns a zero-value
// config (no error). Handles two migrations:
//   - `[source] path = ...` (renamed to `[path] source = ...`) is
//     lifted into Path.Source when the latter is unset.
//   - The pre-config source-path single-line file at
//     xdgConfigDir("work")/source-path is read when nothing else has
//     populated Path.Source.
//
// Fields that start with "~/" are expanded to $HOME.
func loadConfig() (config, error) {
	var c config
	data, err := os.ReadFile(configPath())
	switch {
	case err == nil:
		if err := toml.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("parse %s: %w", configPath(), err)
		}
	case !os.IsNotExist(err):
		return c, fmt.Errorf("read %s: %w", configPath(), err)
	}
	if c.Path.Source == "" && c.LegacySource.Path != "" {
		c.Path.Source = c.LegacySource.Path
	}
	c.LegacySource = legacySourceConfig{}
	if c.Path.Source == "" {
		if legacy := readLegacySourcePath(); legacy != "" {
			c.Path.Source = legacy
		}
	}
	c.Path.Source = expandTilde(c.Path.Source)
	c.Path.Worktrees = expandTilde(c.Path.Worktrees)
	c.Path.Tasks = expandTilde(c.Path.Tasks)
	c.Path.Repos = expandTilde(c.Path.Repos)
	return c, nil
}

// expandTilde returns s with a leading "~/" replaced by $HOME. Empty
// input passes through. Non-tilde paths pass through unchanged.
func expandTilde(s string) string {
	if s == "" || !strings.HasPrefix(s, "~/") {
		return s
	}
	return path.Join(os.Getenv("HOME"), s[2:])
}

// saveConfig writes the config atomically (write to temp + rename).
func saveConfig(c config) error {
	p := configPath()
	if err := os.MkdirAll(path.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", path.Dir(p), err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, p, err)
	}
	return nil
}

// readLegacySourcePath returns the value from the pre-config source-path file
// if it still exists, "" otherwise. install.persistSource now writes into
// config.toml directly, so this only fires during migration.
func readLegacySourcePath() string {
	data, err := os.ReadFile(sourcePathFile())
	if err != nil {
		return ""
	}
	s := string(data)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
