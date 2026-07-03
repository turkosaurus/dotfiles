package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

type statusCmd struct {
	// filterSpec provides -o/-w/-W/-c (dual-use: target status *and*
	// implicit self-exclusion input), -t/-b (picker type filter), and
	// -a/--all (include closed items in the picker).
	filterSpec

	// due-date override; empty → no change
	Due string `arg:"-d,--due" help:"set selected → due (2h, 3d, tomorrow, ...)"`

	// Direct target: '.' → current worktree, else resolve as a worktree
	// name. When set, skip the picker and apply the change to just that
	// one item.
	Name string `arg:"positional" help:"worktree name or '.' (empty → multiselect picker)"`
}

// target resolves the status flag (may be empty statusKind if none set)
// and the due-date override (zero time.Time if --due not set). Returns
// an error when neither is set or when parsing fails.
func (c *statusCmd) target() (statusKind, time.Time, error) {
	s, err := pickStatusFlag(c.Open, c.Waiting, c.Working, c.Closed, "")
	if err != nil {
		return "", time.Time{}, fmt.Errorf("status: %w", err)
	}
	var due time.Time
	if c.Due != "" {
		due, err = parseDue(c.Due)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("status: %w", err)
		}
	}
	if s == "" && due.IsZero() {
		return "", time.Time{}, fmt.Errorf("status: need one of -o/-w/-W/-c or --due")
	}
	return s, due, nil
}

// pickStatusFlag resolves at most one of the four status booleans into a
// statusKind. Zero flags returns fallback (the caller's default). More than
// one flag returns an error.
func pickStatusFlag(open, waiting, working, closed bool, fallback statusKind) (statusKind, error) {
	var picks []statusKind
	if open {
		picks = append(picks, statusOpen)
	}
	if waiting {
		picks = append(picks, statusWaiting)
	}
	if working {
		picks = append(picks, statusWorking)
	}
	if closed {
		picks = append(picks, statusClosed)
	}
	switch len(picks) {
	case 0:
		return fallback, nil
	case 1:
		return picks[0], nil
	default:
		return "", fmt.Errorf("only one of -o/-w/-W/-c may be set")
	}
}

// runStatus prompts multiselect over the filtered inventory and applies
// the target status and/or due-date to each selected item. When a name
// (or ".") is passed as a positional, the picker is bypassed and the
// changes are applied to just that worktree.
func runStatus(c *statusCmd) error {
	target, due, err := c.target()
	if err != nil {
		return err
	}

	if c.Name != "" {
		wt, err := selectWorktree(c.Name)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		it := inventoryItem{Worktree: &wt}
		if err := applyStatusDue(it, target, due); err != nil {
			return fmt.Errorf("status: %w", err)
		}
		pterm.Success.Printfln("%s → %s", wt, describeChange(target, due))
		warnIfBroken()
		return nil
	}

	showWT, showCh := c.showKinds()

	spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
	items, err := loadInventory(showWT, showCh)
	_ = spinner.Stop()
	if err != nil {
		return err
	}
	items = applySprintFilter(items)
	// Hide closed unless --all (same default as list/pick), then exclude
	// items already at target (a set-to-self no-op). Both filters degrade
	// gracefully: --all leaves closed in; target=="" leaves everything in.
	if !c.All {
		items = excludeStatus(items, statusClosed)
	}
	items = excludeAtStatus(items, target)
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

	change := describeChange(target, due)
	if !confirm(fmt.Sprintf("apply to %d items → %s?", len(sel), change)) {
		return fmt.Errorf("status: cancelled")
	}

	for _, label := range sel {
		it := byLabel[label]
		if err := applyStatusDue(it, target, due); err != nil {
			pterm.Warning.Printfln("FAIL %s: %v", label, err)
			continue
		}
		pterm.Success.Printfln("%s → %s", label, change)
	}
	warnIfBroken()
	return nil
}

// describeChange formats the "→ ..." tail for status/due output. Emits
// "status=<s>", "due=<t>", or both joined by a comma.
func describeChange(target statusKind, due time.Time) string {
	switch {
	case target != "" && !due.IsZero():
		return fmt.Sprintf("status=%s, due=%s", target, due.Format("2006-01-02 15:04"))
	case target != "":
		return string(target)
	case !due.IsZero():
		return fmt.Sprintf("due=%s", due.Format("2006-01-02 15:04"))
	}
	return "(no change)"
}

// applyStatusDue applies whichever of status/due is set to the item.
// Status changes go through setStatus (moveTask for tasks, yq for
// worktrees). Due changes rewrite plan.toml directly (tasks) or yq
// the worktree plan. Both fields can be applied in one call.
func applyStatusDue(it inventoryItem, target statusKind, due time.Time) error {
	if target != "" {
		if err := setStatus(it, target); err != nil {
			return err
		}
	}
	if !due.IsZero() {
		if err := setDue(it, target, due); err != nil {
			return err
		}
	}
	return nil
}

// setDue writes the due-date to the plan file. For tasks, target may be
// non-empty when setStatus just moved the file — resolve the new path
// via taskDir(target) rather than the stale it.Task.Path.
func setDue(it inventoryItem, movedTo statusKind, due time.Time) error {
	planPath := ""
	switch {
	case it.Task != nil:
		if movedTo != "" {
			planPath = path.Join(taskDir(movedTo), path.Base(it.Task.Path))
		} else {
			planPath = it.Task.Path
		}
	case it.Worktree != nil:
		planPath = path.Join(it.Worktree.Path, planFileName)
		if _, err := ensurePlanFile(planPath, it.Worktree.String(), it.Worktree.Branch); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown item type")
	}
	// yq encodes RFC3339 strings as native TOML datetime on output.
	stamp := due.Format(time.RFC3339)
	cmd := exec.Command("yq", "-p", "toml", "-o", "toml", "-i",
		fmt.Sprintf(`.due = "%s"`, stamp), planPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yq due: %w", err)
	}
	return nil
}

// setStatus applies target to one item.
//   - task: moveTask (renames file into ~/w/t/<target>/)
//   - worktree: yq-edit plan.toml's status; if missing, prompt to seed
//     via ensurePlanFile (respects --yes)
func setStatus(it inventoryItem, target statusKind) error {
	switch {
	case it.Task != nil:
		_, err := moveTask(*it.Task, target)
		return err

	case it.Worktree != nil:
		planPath := path.Join(it.Worktree.Path, planFileName)
		if _, err := ensurePlanFile(planPath, it.Worktree.String(), it.Worktree.Branch); err != nil {
			return err
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
