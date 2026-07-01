package main

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
)

type pickCmd struct {
	Name      string `arg:"positional" help:"branch name to navigate to; empty → fuzzy-pick"`
	Chores    bool   `arg:"-c,--chores" help:"only offer chores in the picker"`
	Worktrees bool   `arg:"-w,--worktrees" help:"only offer worktrees in the picker"`
}
type mainCmd struct{} // work main
type prevCmd struct{} // work -

// runPick presents a unified interactive select over worktrees + chores when
// no name is given. Selecting a worktree emits its path; selecting a chore
// opens the chore file in $EDITOR. When a name is given, only worktrees are
// matched (chore numbers can be added later if desired).
func runPick(c *pickCmd) error {
	if c.Name == "" {
		showWT := !c.Chores || c.Worktrees
		showCh := !c.Worktrees || c.Chores
		spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
		items, err := loadInventory(showWT, showCh)
		_ = spinner.Stop()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("nothing to pick under %s", defaultWorkDir)
		}
		it, err := pickInventory(items)
		if err != nil {
			return err
		}
		switch {
		case it.Worktree != nil:
			emitPath(it.Worktree.Path)
			return nil
		case it.Chore != nil:
			return openInEditor(it.Chore.Path)
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

// runPrev emits the previously-visited path saved by emitPath. Doesn't call
// emitPath — that would overwrite the .previous file, making `work -` a loop.
func runPrev(_ *prevCmd) error {
	p, err := readPrevious()
	if err != nil {
		return fmt.Errorf("prev: %w", err)
	}
	if realStdout != nil {
		fmt.Fprintln(realStdout, p)
	} else {
		fmt.Println(p)
	}
	return nil
}

// branchSlug replaces '/' with '-' in a branch name for use as a directory name.
func branchSlug(b string) string {
	return strings.ReplaceAll(b, "/", "-")
}
