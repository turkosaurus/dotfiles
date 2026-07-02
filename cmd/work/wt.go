package main

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

type pickCmd struct {
	Name string `arg:"positional" help:"branch name to navigate to; empty → fuzzy-pick"`

	// type filters
	Tasks     bool `arg:"-t,--task" help:"offer only tasks in the picker"`
	Worktrees bool `arg:"-b,--branch" help:"offer only worktree branches in the picker"`

	// status filters — combinable
	Open    bool `arg:"-o,--open" help:"only offer items with status=open"`
	Waiting bool `arg:"-w,--waiting" help:"only offer items with status=waiting"`
	Working bool `arg:"-W,--working" help:"only offer items with status=working"`
	Closed  bool `arg:"-c,--closed" help:"only offer items with status=closed"`
}

// statusFilter mirrors listCmd.statusFilter().
func (c *pickCmd) statusFilter() map[statusKind]bool {
	set := map[statusKind]bool{}
	if c.Open {
		set[statusOpen] = true
	}
	if c.Waiting {
		set[statusWaiting] = true
	}
	if c.Working {
		set[statusWorking] = true
	}
	if c.Closed {
		set[statusClosed] = true
	}
	if len(set) == 0 {
		return nil
	}
	return set
}
type mainCmd struct{} // work main
type prevCmd struct{} // work -

// runPick presents a unified interactive select over worktrees + tasks when
// no name is given. Selecting a worktree emits its path; selecting a task
// opens the task file in $EDITOR. When a name is given, only worktrees are
// matched (task numbers can be added later if desired).
func runPick(c *pickCmd) error {
	if c.Name == "" {
		showWT := !c.Tasks || c.Worktrees
		showCh := !c.Worktrees || c.Tasks
		spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
		items, err := loadInventory(showWT, showCh)
		_ = spinner.Stop()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("nothing under %s", defaultWorkDir)
		}
		items = filterByStatus(items, c.statusFilter())
		if len(items) == 0 {
			return fmt.Errorf("no items match the current filters")
		}
		it, err := pickInventory(items)
		if err != nil {
			return err
		}
		switch {
		case it.Worktree != nil:
			emitPath(it.Worktree.Path)
			return nil
		case it.Task != nil:
			return openInEditor(it.Task.Path)
		}
		return fmt.Errorf("pick: unknown item")
	}
	slug := branchSlug(c.Name)
	if dir := findWorktree(slug); dir != "" {
		emitPath(dir)
		return nil
	}
	return fmt.Errorf("no worktree for %q — switch to that branch in the main worktree and run `work new .`", c.Name)
}

// runMain emits the main worktree path for the current repo.
func runMain(_ *mainCmd) error {
	if p := mainWorktreePath("."); p != "" {
		emitPath(p)
		return nil
	}
	// Fallback: scan ~/w/<repo>/* for any worktree, ask git from there.
	repo, err := currentRepoName()
	if err != nil {
		return fmt.Errorf("main: %w", err)
	}
	wts, err := listWorktrees()
	if err != nil {
		return err
	}
	for _, wt := range wts {
		if wt.Repo != repo {
			continue
		}
		if p := mainWorktreePath(wt.Path); p != "" {
			emitPath(p)
			return nil
		}
	}
	return fmt.Errorf("main worktree not found")
}

// runPrev writes the previously-visited path (saved by emitPath) as the
// next cd-target. Uses writeNextPath, not emitPath, so we don't overwrite
// .previous with the cwd — that would make `work -` a two-step loop.
func runPrev(_ *prevCmd) error {
	p, err := readPrevious()
	if err != nil {
		return fmt.Errorf("prev: %w", err)
	}
	writeNextPath(p)
	return nil
}

// branchSlug replaces '/' with '-' in a branch name for use as a directory name.
func branchSlug(b string) string {
	return strings.ReplaceAll(b, "/", "-")
}
