package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type editCmd struct {
	Arg string `arg:"positional" help:"'.' for current worktree; empty → picker"`
	filterSpec
}

// runEdit has two modes:
//   - "."   → open the current worktree's plan.toml in $EDITOR
//   - empty → picker over filtered items → open chosen plan.toml
//
// Type/status filters and --all (include closed) apply to picker mode
// via the embedded filterSpec.
func runEdit(c *editCmd) error {
	if c.Arg == "." {
		return editCurrentPlan()
	}
	if c.Arg != "" {
		return fmt.Errorf(`edit: expected "." or empty; got %q`, c.Arg)
	}
	return editPickedPlan(c)
}

// editPickedPlan shows a picker over the filtered inventory and opens
// the selected item's plan.toml in $EDITOR. Worktree plan.toml files
// are created on demand if missing; task files always already exist.
func editPickedPlan(c *editCmd) error {
	showWT, showCh := c.showKinds()
	items, err := loadInventory(showWT, showCh)
	if err != nil {
		return err
	}
	set, explicit := c.statusFilter()
	items = filterInventory(items, set, explicit)
	if len(items) == 0 {
		pterm.Info.Println("nothing to edit")
		return nil
	}
	it, err := pickInventory(items)
	if err != nil {
		return err
	}
	var planPath string
	switch {
	case it.Worktree != nil:
		planPath = path.Join(it.Worktree.Path, planFileName)
		if _, err := ensurePlanFile(planPath, it.Worktree.String(), it.Worktree.Branch); err != nil {
			return fmt.Errorf("edit: %w", err)
		}
	case it.Task != nil:
		planPath = it.Task.Path
	default:
		return fmt.Errorf("edit: unknown item")
	}
	if err := openInEditor(planPath); err != nil {
		return err
	}
	if _, err := readPlan(planPath); err != nil {
		pterm.Error.Printfln("%v", err)
		return errPrinted
	}
	return nil
}

// editCurrentPlan opens the plan.toml of the current worktree in $EDITOR,
// then validates it and reports parse errors immediately.
func editCurrentPlan() error {
	wt, err := currentWorktree()
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}
	planPath := path.Join(wt.Path, planFileName)
	if _, err := ensurePlanFile(planPath, wt.String(), wt.Branch); err != nil {
		return fmt.Errorf("edit: %w", err)
	}
	if err := openInEditor(planPath); err != nil {
		return err
	}
	if _, err := readPlan(planPath); err != nil {
		pterm.Error.Printfln("%v", err)
		return errPrinted
	}
	return nil
}

// itemStatus returns the current status of an item (worktree reads plan.toml).
func itemStatus(it inventoryItem) statusKind {
	switch {
	case it.Task != nil:
		return it.Task.Status
	case it.Worktree != nil:
		if p, err := readPlan(path.Join(it.Worktree.Path, planFileName)); err == nil {
			return p.Status
		}
		return statusOpen
	}
	return statusOpen
}

// itemLabel returns the user-facing name of an item.
func itemLabel(it inventoryItem) string {
	switch {
	case it.Worktree != nil:
		return it.Worktree.String()
	case it.Task != nil:
		if it.Task.Title != "" {
			return it.Task.Title
		}
		return strings.TrimSuffix(path.Base(it.Task.Path), ".toml")
	}
	return ""
}
