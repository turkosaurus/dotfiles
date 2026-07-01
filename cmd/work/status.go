package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

type statusCmd struct {
	// target status — exactly one required
	Open    bool `arg:"-o,--open" help:"set selected → status=open"`
	Waiting bool `arg:"-w,--waiting" help:"set selected → status=waiting"`
	Working bool `arg:"-W,--working" help:"set selected → status=working"`
	Closed  bool `arg:"-c,--closed" help:"set selected → status=closed"`

	// type filter for the picker
	Tasks     bool `arg:"-t,--task" help:"offer only tasks in the picker"`
	Worktrees bool `arg:"-b,--branch" help:"offer only worktree branches in the picker"`
}

// target returns the single target status from the flags, or an error if
// zero or more than one flag is set.
func (c *statusCmd) target() (statusKind, error) {
	var picks []statusKind
	if c.Open {
		picks = append(picks, statusOpen)
	}
	if c.Waiting {
		picks = append(picks, statusWaiting)
	}
	if c.Working {
		picks = append(picks, statusWorking)
	}
	if c.Closed {
		picks = append(picks, statusClosed)
	}
	switch len(picks) {
	case 0:
		return "", fmt.Errorf("status: need exactly one of -o/-w/-W/-c")
	case 1:
		return picks[0], nil
	default:
		return "", fmt.Errorf("status: only one target may be set")
	}
}

// runStatus prompts multiselect over the filtered inventory and applies the
// target status to each selected item.
func runStatus(c *statusCmd) error {
	target, err := c.target()
	if err != nil {
		return err
	}

	showWT := !c.Tasks || c.Worktrees
	showCh := !c.Worktrees || c.Tasks

	spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
	items, err := loadInventory(showWT, showCh)
	_ = spinner.Stop()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		pterm.Info.Println("nothing to update")
		return nil
	}

	labels := formatLabels(items)
	byLabel := make(map[string]inventoryItem, len(items))
	for i, it := range items {
		byLabel[labels[i]] = it
	}

	// Space is reserved for the filter's text input, so we can't use it for
	// toggle. Swap pterm's defaults so Enter confirms (matching shell
	// intuition) and Tab toggles.
	sel, err := pterm.DefaultInteractiveMultiselect.
		WithOptions(labels).
		WithFilter(true).
		WithMaxHeight(20).
		WithKeySelect(keys.Tab).
		WithKeyConfirm(keys.Enter).
		Show()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if len(sel) == 0 {
		pterm.Info.Println("nothing selected")
		return nil
	}

	if !confirm(fmt.Sprintf("mark %d items as %s?", len(sel), target)) {
		return fmt.Errorf("status: cancelled")
	}

	for _, label := range sel {
		it := byLabel[label]
		if err := setStatus(it, target); err != nil {
			pterm.Warning.Printfln("FAIL %s: %v", label, err)
			continue
		}
		pterm.Success.Printfln("%s → %s", label, target)
	}
	warnIfBroken()
	return nil
}

// setStatus applies target to one item.
//   - task: moveTask (renames file into ~/w/t/<target>/)
//   - worktree: yq-edit plan.toml's status; if missing, prompt to seed
//     (always prompts, even with --yes, since seeding is a file-creating action)
func setStatus(it inventoryItem, target statusKind) error {
	switch {
	case it.Task != nil:
		_, err := moveTask(*it.Task, target)
		return err

	case it.Worktree != nil:
		planPath := path.Join(it.Worktree.Path, planFileName)
		if _, err := os.Stat(planPath); os.IsNotExist(err) {
			if !confirmAlways(fmt.Sprintf("no plan.toml in %s. Seed one?", it.Worktree)) {
				return fmt.Errorf("skipped (no plan.toml)")
			}
			if err := seedPlan(planPath, it.Worktree.Branch); err != nil {
				return fmt.Errorf("seed: %w", err)
			}
		}
		cmd := exec.Command("yq", "-p", "toml", "-o", "toml", "-i",
			fmt.Sprintf(`.status = "%s"`, target), planPath)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("yq: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unknown item type")
}
