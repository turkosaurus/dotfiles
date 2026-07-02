package main

import (
	"fmt"
	"os"
	"path"

	"github.com/pelletier/go-toml/v2"
	"github.com/pterm/pterm"
)

// defaultConfigTemplate is the seeded content for a fresh config.toml. TOML
// supports # line comments, so each field carries an inline explanation for
// users editing via `work config`. Keep formatting simple — go-toml won't
// preserve these comments if the file is ever rewritten via saveConfig, so
// treat them as install-time guidance, not durable metadata.
const defaultConfigTemplate = `# work config — per-user settings. Edited with 'work config'.
# Lives at $XDG_CONFIG_HOME/work/config.toml (fallback: ~/.config/work/config.toml).

[source]
# Module directory (contains go.mod) that 'work install' rebuilds from.
# Populated automatically the first time you run 'work install'.
path = ""

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
	Source sourceConfig `toml:"source"`
	Sprint sprintConfig `toml:"sprint"`
}

type sourceConfig struct {
	// Path is the module directory (containing go.mod) used by `work install`
	// to rebuild the binary. Set on first install.
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
//	project_url = "https://github.com/orgs/getlantern/projects/127"
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

// configPath is $XDG_CONFIG_HOME/work/config.toml, falling back to
// $HOME/.config/work/config.toml.
func configPath() string {
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

// loadConfig reads the config file. Missing file returns a zero-value config
// (no error). Also migrates the legacy source-path single-line file if it
// still exists and config.toml doesn't yet have a source.path.
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
	if c.Source.Path == "" {
		if legacy := readLegacySourcePath(); legacy != "" {
			c.Source.Path = legacy
		}
	}
	return c, nil
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
