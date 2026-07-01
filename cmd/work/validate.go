package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
)

type validateCmd struct {
	Arg string `arg:"positional" help:"'.' for current worktree's plan.toml (default)"`
	All bool   `arg:"-a,--all" help:"validate every plan under ~/w"`
}

type brokenPlan struct {
	path string
	err  error
}

// runValidate has two modes:
//   - default / "." → validate the current worktree's plan.toml
//   - --all         → walk every plan.toml under ~/w and report errors
func runValidate(c *validateCmd) error {
	if !c.All {
		if c.Arg != "" && c.Arg != "." {
			return fmt.Errorf(`validate: expected "." or --all; got %q`, c.Arg)
		}
		return validateCurrent()
	}
	return validateAll()
}

// validateCurrent parses the current worktree's plan.toml and reports.
func validateCurrent() error {
	wt, err := currentWorktree()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	planPath := path.Join(wt.Path, planFileName)
	if _, err := readPlan(planPath); err != nil {
		pterm.Error.Printfln("%s", relPath(planPath))
		pterm.Error.Printfln("  %v", err)
		return fmt.Errorf("plan parse error")
	}
	pterm.Success.Printfln("%s: ok", wt)
	return nil
}

// validateAll walks every plan.toml under ~/w and prints parse errors.
func validateAll() error {
	broken, err := findBrokenPlans()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if len(broken) == 0 {
		pterm.Success.Println("all plans parse ok")
		return nil
	}
	for _, b := range broken {
		pterm.Error.Printfln("%s", relPath(b.path))
		pterm.Error.Printfln("  %v", b.err)
	}
	return fmt.Errorf("%d plan(s) have parse errors", len(broken))
}

// findBrokenPlans scans all plan.toml files (worktrees + tasks) and returns
// those that fail to parse. Missing files are not broken.
func findBrokenPlans() ([]brokenPlan, error) {
	var out []brokenPlan

	// Worktree plans: ~/w/<repo>/<branch>/plan.toml (skip anything under task root)
	wtMatches, err := filepath.Glob(path.Join(defaultWorkDir, "*", "*", planFileName))
	if err != nil {
		return nil, fmt.Errorf("glob worktree plans: %w", err)
	}
	for _, m := range wtMatches {
		if strings.HasPrefix(m, defaultTaskDir+"/") {
			continue
		}
		if _, err := readPlan(m); err != nil {
			out = append(out, brokenPlan{path: m, err: err})
		}
	}

	// Task plans: ~/w/t/<status>/*.toml
	tMatches, err := filepath.Glob(path.Join(defaultTaskDir, "*", "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("glob task plans: %w", err)
	}
	for _, m := range tMatches {
		if _, err := readPlan(m); err != nil {
			out = append(out, brokenPlan{path: m, err: err})
		}
	}

	return out, nil
}

// warnIfBroken prints a one-line warning if any plan is broken. Called
// automatically at the end of mutating commands (sync, edit, status) so the
// user doesn't have to remember to run `work validate`.
func warnIfBroken() {
	broken, err := findBrokenPlans()
	if err != nil || len(broken) == 0 {
		return
	}
	pterm.Warning.Printfln("%d plan(s) have parse errors — run `work validate` for details", len(broken))
}
