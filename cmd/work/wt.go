package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type pickCmd struct {
	Name string `arg:"positional" help:"branch name to navigate to; empty → fuzzy-pick"`
	filterSpec
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
		items, err := loadInventory(showWT, showCh)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("nothing under %s", defaultWorkDir)
		}
		set, explicit := c.statusFilter()
		items = filterInventory(items, set, explicit)
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

// runMain emits the main worktree path for the current repo. Outside a
// repo, or when its main tree can't be found, it falls back to the
// worktree picker rather than failing.
func runMain(_ *mainCmd) error {
	if p := mainWorktreePath("."); p != "" {
		emitPath(p)
		return nil
	}
	// Fallback: scan ~/w/<repo>/* for any worktree, ask git from there.
	repo := cwdRepoName()
	if repo == "" {
		pterm.Info.Println("main: not in a repo; pick a worktree")
		return runPick(&pickCmd{filterSpec: filterSpec{Worktrees: true}})
	}
	wts, err := listWorktrees()
	if err != nil {
		return fmt.Errorf("main: %w", err)
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
	pterm.Info.Printfln("main: no main worktree for %s; pick one", repo)
	return runPick(&pickCmd{filterSpec: filterSpec{Worktrees: true}})
}

// cwdRepoName returns the repo for cwd: from git when inside one,
// else from a ~/w/<repo>/... path (e.g. a removed worktree's parent).
// Returns "" when neither applies.
func cwdRepoName() string {
	if repo, err := currentRepoName(); err == nil {
		return repo
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	rel, ok := strings.CutPrefix(cwd, defaultWorkDir+"/")
	if !ok {
		return ""
	}
	repo := strings.SplitN(rel, "/", 2)[0]
	if path.Join(defaultWorkDir, repo) == defaultTaskDir {
		return ""
	}
	return repo
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
