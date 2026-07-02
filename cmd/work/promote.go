package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

type promoteCmd struct{}

// runPromote presents a tasks-only multiselect picker and folds all selected
// tasks into a new worktree from the current branch (like `work new .`). The
// first selection's plan.toml becomes the worktree's plan; remaining
// selections merge in (tasks[], [[issue]], [[pr]]) and get moved to closed.
//
// Run from the target repo's main worktree, checked out on the branch you
// want the promoted work to live on. main/master is refused.
func runPromote(_ *promoteCmd) error {
	tasks, err := listTasksAll()
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	items := make([]inventoryItem, 0, len(tasks))
	for i := range tasks {
		if tasks[i].Status == statusClosed {
			continue
		}
		t := tasks[i]
		items = append(items, inventoryItem{Task: &t})
	}
	if len(items) == 0 {
		pterm.Info.Println("promote: no open tasks")
		return nil
	}

	labels := formatLabels(items)
	byLabel := make(map[string]inventoryItem, len(items))
	for i, it := range items {
		byLabel[labels[i]] = it
	}
	sel, err := pterm.DefaultInteractiveMultiselect.
		WithOptions(labels).
		WithFilter(true).
		WithMaxHeight(20).
		WithKeySelect(keys.Tab).
		WithKeyConfirm(keys.Enter).
		Show()
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	if len(sel) == 0 {
		pterm.Info.Println("promote: nothing selected")
		return nil
	}
	picks := make([]plan, 0, len(sel))
	for _, label := range sel {
		it := byLabel[label]
		if it.Task == nil {
			continue
		}
		picks = append(picks, *it.Task)
	}

	branch, err := currentBranch(".")
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("promote: refusing to promote onto %s; switch to a feature branch first", branch)
	}
	repo, err := currentRepoName()
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	dir := path.Join(defaultWorkDir, repo, branchSlug(branch))
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("promote: worktree already exists: %s", dir)
	}
	if err := os.MkdirAll(path.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("promote: mkdir: %w", err)
	}
	// Switch current checkout back to main/master so the branch is free.
	if err := exec.Command("git", "switch", "main").Run(); err != nil {
		if err2 := exec.Command("git", "switch", "master").Run(); err2 != nil {
			return fmt.Errorf("promote: could not switch back to main/master")
		}
	}
	cmd := exec.Command("git", "worktree", "add", dir, branch)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("promote: git worktree add: %w", err)
	}

	// First pick becomes the worktree's plan.toml (rename → keep file lineage).
	primary := picks[0]
	dst := path.Join(dir, planFileName)
	if err := os.Rename(primary.Path, dst); err != nil {
		return fmt.Errorf("promote: move task → plan: %w", err)
	}
	primary.Status = statusWorking
	primary.Path = dst

	// Fold remaining picks in, then close them.
	for _, other := range picks[1:] {
		src, err := readPlan(other.Path)
		if err != nil {
			pterm.Warning.Printfln("skip %s: %v", relPath(other.Path), err)
			continue
		}
		mergePlanFields(&primary, src)
		if _, err := moveTask(other, statusClosed); err != nil {
			pterm.Warning.Printfln("close %s: %v", relPath(other.Path), err)
			continue
		}
		pterm.Success.Printfln("folded in %s", relPath(other.Path))
	}
	if err := writePlan(primary); err != nil {
		return fmt.Errorf("promote: write plan: %w", err)
	}
	pterm.Success.Printfln("promoted %d task(s) → %s", len(picks), relPath(dir))
	emitPath(dir)
	return nil
}
