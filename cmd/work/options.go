package main

import (
	"errors"
	"io"
	"os"
	"path"

	"github.com/pterm/pterm"
)

// errPrinted signals to main that a subcommand already reported the failure,
// so main should exit non-zero without re-printing. Its Error() must be empty.
var errPrinted = errors.New("")

// log is the structured debug logger. Default level is Info (debug filtered out);
// main flips it to Debug when -v is set. All user-facing output stays on the
// print-style printers (pterm.Info, Success, Warning, Error) for visual parity
// with pickers and tables.
var log = pterm.DefaultLogger.WithLevel(pterm.LogLevelInfo).WithTime(false)

var (
	confirmYes      bool   // set from --yes; bypasses confirmation prompts
	quietMode       bool   // set from -q/--quiet; suppresses INFO and SUCCESS output
	verboseMode     bool   // set from -v/--verbose; unlocks noisier reporting (e.g. sprint's ignored-column breakdown)
	sprintFilterURL string // set from -s/--sprint; only items linked to this project URL survive the picker filter
	// defaultWorkDir / defaultTaskDir are populated once at startup by
	// applyPathConfig (called from setupDirs). Values here are the fallback
	// when config.toml doesn't set path.worktrees / path.tasks.
	defaultWorkDir = path.Join(os.Getenv("HOME"), "w")
	defaultTaskDir = path.Join(defaultWorkDir, "t")
)

// applyPathConfig overrides defaultWorkDir/defaultTaskDir from the loaded
// config. Called from setupDirs so all path-consuming code sees the
// user's choices without needing to plumb config through.
func applyPathConfig(c config) {
	if c.Path.Worktrees != "" {
		defaultWorkDir = c.Path.Worktrees
	}
	if c.Path.Tasks != "" {
		defaultTaskDir = c.Path.Tasks
	} else {
		defaultTaskDir = path.Join(defaultWorkDir, "t")
	}
}

// setSprintFilter loads the configured sprint project URL and stores it
// in sprintFilterURL. Applied by applySprintFilter on every loadInventory
// call. If the config has no project_url, the filter stays inert (all
// items pass) rather than silently hiding everything.
func setSprintFilter() {
	c, err := loadConfig()
	if err != nil || c.Sprint.ProjectURL == "" {
		return
	}
	sprintFilterURL = c.Sprint.ProjectURL
}

// setQuietMode routes INFO/SUCCESS pterm output to io.Discard and flips
// the quietMode global so buffered output paths can skip low-severity
// lines at collect time. WARN and ERROR still hit stderr.
func setQuietMode() {
	quietMode = true
	pterm.Info.Writer = io.Discard
	pterm.Success.Writer = io.Discard
}

// hasSprintLink reports whether the item's plan has any [[issue]] whose
// project.url matches the configured sprint project URL. Used by
// applySprintFilter.
func hasSprintLink(it inventoryItem) bool {
	var issues []Issue
	switch {
	case it.Task != nil:
		issues = it.Task.Issues
	case it.Worktree != nil:
		p, err := readPlan(path.Join(it.Worktree.Path, planFileName))
		if err != nil {
			return false
		}
		issues = p.Issues
	}
	for _, i := range issues {
		if i.Project.URL == sprintFilterURL {
			return true
		}
	}
	return false
}

// applySprintFilter narrows items to those linked to the configured
// sprint project. No-op when -s/--sprint wasn't set (sprintFilterURL "").
func applySprintFilter(items []inventoryItem) []inventoryItem {
	if sprintFilterURL == "" {
		return items
	}
	out := items[:0]
	for _, it := range items {
		if hasSprintLink(it) {
			out = append(out, it)
		}
	}
	return out
}

// excludeAtStatus drops items whose current status equals target — the
// "no-op set" filter. Only meaningful for commands with a target status
// (like `set`/`status`); pass "" to disable.
func excludeAtStatus(items []inventoryItem, target statusKind) []inventoryItem {
	if target == "" {
		return items
	}
	return excludeStatus(items, target)
}

// excludeStatus drops items whose current status equals kind. Distinct
// from excludeAtStatus in that the caller has already decided to filter
// (no empty-string escape hatch). Used by `set` to hide closed items by
// default without leaning on filterInventory's status set.
func excludeStatus(items []inventoryItem, kind statusKind) []inventoryItem {
	out := items[:0]
	for _, it := range items {
		if itemStatus(it) != kind {
			out = append(out, it)
		}
	}
	return out
}

// filterInventory applies status + sprint filters. Composition rule for
// -s/--sprint depends on whether status flags were set explicitly:
//
//   - statusExplicit=true, -s active  → union: status ∪ non-closed sprint
//     (e.g. `list -wW -s` = waiting + working + everything else in sprint)
//   - statusExplicit=false, -s active → intersect: status ∩ sprint
//     (e.g. `list -s` = default {open,waiting,working} narrowed to sprint)
//   - -s inactive                     → just status
//
// The union case skips closed sprint items unless the caller's status
// set already included statusClosed (via `--all` or `-c`).
func filterInventory(items []inventoryItem, statusSet map[statusKind]bool, statusExplicit bool) []inventoryItem {
	statusFiltered := filterByStatus(items, statusSet)
	if sprintFilterURL == "" {
		return statusFiltered
	}
	if !statusExplicit {
		return applySprintFilter(statusFiltered)
	}
	includeClosed := statusSet == nil || statusSet[statusClosed]
	seen := make(map[string]bool, len(statusFiltered))
	for _, it := range statusFiltered {
		seen[it.key()] = true
	}
	for _, it := range items {
		if seen[it.key()] {
			continue
		}
		if !includeClosed && itemStatus(it) == statusClosed {
			continue
		}
		if !hasSprintLink(it) {
			continue
		}
		statusFiltered = append(statusFiltered, it)
	}
	return statusFiltered
}

func init() {
	// Badge texts padded to 5 chars, shorter words centered.
	pterm.Debug.Prefix.Text = "DEBUG"
	pterm.Info.Prefix.Text = "INFO "
	pterm.Warning.Prefix.Text = "WARN "
	pterm.Error.Prefix.Text = "ERROR"
	pterm.Success.Prefix.Text = " OK  "

	// Route all pterm output to stderr so stdout is reserved for cd-target
	// paths that the shell wrapper consumes.
	pterm.Debug.Writer = os.Stderr
	pterm.Info.Writer = os.Stderr
	pterm.Warning.Writer = os.Stderr
	pterm.Error.Writer = os.Stderr
	pterm.Success.Writer = os.Stderr
	pterm.DefaultSpinner.Writer = os.Stderr
	pterm.DefaultProgressbar.Writer = os.Stderr
}
