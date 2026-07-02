package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

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
	confirmYes         bool   // set from --yes; bypasses confirmation prompts
	quietMode          bool   // set from -q/--quiet; suppresses INFO and SUCCESS output
	verboseMode        bool   // set from -v/--verbose; unlocks noisier reporting (e.g. sprint's ignored-column breakdown)
	projectFilter      *bool  // set from -p/--project; nil = no filter, true = must have a project link, false = must have none
	sprintFilterURL    string // set from -s/--sprint; only items linked to this project URL survive the picker filter
	defaultWorkDir            = path.Join(os.Getenv("HOME"), "w")
	defaultTaskDir            = path.Join(defaultWorkDir, "t")
	defaultDaysDue             = 3
)

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

// setProjectFilter parses -p/--project. Accepts "" (unset), "t"/"true", or
// "f"/"false" (case-insensitive). Anything else is a user error.
func setProjectFilter(raw string) error {
	if raw == "" {
		projectFilter = nil
		return nil
	}
	switch strings.ToLower(raw) {
	case "t", "true":
		v := true
		projectFilter = &v
	case "f", "false":
		v := false
		projectFilter = &v
	default:
		return fmt.Errorf("--project: expected t/true or f/false, got %q", raw)
	}
	return nil
}

// hasProjectLink reports whether the item's plan has any [[issue]] with a
// non-empty project.url. Used by applyProjectFilter.
func hasProjectLink(it inventoryItem) bool {
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
		if i.Project.URL != "" {
			return true
		}
	}
	return false
}

// applyProjectFilter filters items in place using projectFilter. No-op when
// projectFilter is nil.
func applyProjectFilter(items []inventoryItem) []inventoryItem {
	if projectFilter == nil {
		return items
	}
	want := *projectFilter
	out := items[:0]
	for _, it := range items {
		if hasProjectLink(it) == want {
			out = append(out, it)
		}
	}
	return out
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
