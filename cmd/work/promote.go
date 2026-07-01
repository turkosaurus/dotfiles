package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"

	"github.com/pterm/pterm"
)

type promoteCmd struct {
	Num int `arg:"positional,required" help:"task number (as shown in ~/w/t/open/N.toml or via 'work list')"`
}

// runPromote is the inverse of the plan-promotion in `work rm`:
// given a task under ~/w/t/open/<N>.toml, create a worktree from the current
// branch (like `work new .`) and move the task's plan.toml into it as the
// worktree's plan. The task file is deleted on success.
//
// Run this from the target repo's main worktree, checked out on the branch you
// want the task to live on. main/master is refused (same rule as `work new .`).
func runPromote(c *promoteCmd) error {
	src, err := findOpenTask(c.Num)
	if err != nil {
		return fmt.Errorf("promote: %w", err)
	}
	p, err := readPlan(src)
	if err != nil {
		return fmt.Errorf("promote: read task: %w", err)
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

	// Move (don't delete) the task file into the new worktree, then rewrite
	// it in place with status=working and the new path. Rename keeps the file
	// lineage intact — no os.Remove.
	dst := path.Join(dir, planFileName)
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("promote: move task → plan: %w", err)
	}
	p.Status = statusWorking
	p.Path = dst
	if err := writePlan(p); err != nil {
		return fmt.Errorf("promote: write plan: %w", err)
	}

	pterm.Success.Printfln("promoted #%d → %s", c.Num, relPath(dir))
	emitPath(dir)
	return nil
}

// findOpenTask locates an open task by number. Only ~/w/t/open/ is searched —
// closed/waiting/working tasks are not promoted.
func findOpenTask(n int) (string, error) {
	p := path.Join(taskDir(statusOpen), strconv.Itoa(n)+".toml")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	// Fall back to a glob in case the file is padded/named oddly.
	matches, _ := filepath.Glob(path.Join(taskDir(statusOpen), "*.toml"))
	for _, m := range matches {
		if path.Base(m) == strconv.Itoa(n)+".toml" {
			return m, nil
		}
	}
	return "", fmt.Errorf("no open task #%d (looked in %s)", n, taskDir(statusOpen))
}
