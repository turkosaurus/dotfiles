package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type newCmd struct {
	Arg string `arg:"positional" help:"'.' for worktree from current branch; \"title with spaces\" for a task"`

	// initial status for a new task — at most one; default is open
	Open    bool `arg:"-o,--open" help:"new task in open status (default)"`
	Waiting bool `arg:"-w,--waiting" help:"new task in waiting status"`
	Working bool `arg:"-W,--working" help:"new task in working status"`
	Closed  bool `arg:"-c,--closed" help:"new task in closed status"`
}

// runNew dispatches on the shape of Arg:
//   - ""                 → print usage
//   - "."                → worktree from the current branch
//   - contains a space   → task (title stored in ~/w/t/<status>/N.toml), opens $EDITOR
//   - anything else      → error (worktree-from-branch is not supported)
func runNew(c *newCmd) error {
	switch {
	case c.Arg == "":
		pterm.Info.Println(`usage: work new [-o|-w|-W|-c] <arg>

  work new .                          create worktree from the current branch
  work new "title with spaces"        create a task (opens $EDITOR)
  work new -W "title with spaces"     create a task in working status
  work new -w "title with spaces"     create a task in waiting status

Worktrees only come from the current branch — switch branches first, then run
'work new .'. To navigate an existing worktree, use 'work' or 'work <name>'.`)
		return nil
	case c.Arg == ".":
		return newFromCurrent()
	case strings.Contains(c.Arg, " "):
		status, err := pickStatusFlag(c.Open, c.Waiting, c.Working, c.Closed, statusOpen)
		if err != nil {
			return fmt.Errorf("new: %w", err)
		}
		p, err := newTask(c.Arg, status)
		if err != nil {
			return err
		}
		pterm.Success.Printfln("task #%s (%s): %s",
			path.Base(strings.TrimSuffix(p.Path, ".toml")), status, p.Title)
		return openInEditor(p.Path)
	default:
		return fmt.Errorf(`new: expected "." (current branch) or a quoted title with spaces; got %q`, c.Arg)
	}
}

// newFromCurrent moves the current branch to ~/w/<repo>/<slug>/ via git worktree.
func newFromCurrent() error {
	branch, err := currentBranch(".")
	if err != nil {
		return fmt.Errorf("new .: %w", err)
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("already on %s, nothing to move", branch)
	}
	repo, err := currentRepoName()
	if err != nil {
		return fmt.Errorf("new .: %w", err)
	}
	dir := path.Join(defaultWorkDir, repo, branchSlug(branch))
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("worktree already exists: %s", dir)
	}
	if err := os.MkdirAll(path.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// Switch current checkout back to main/master so the branch is free.
	if err := exec.Command("git", "switch", "main").Run(); err != nil {
		if err2 := exec.Command("git", "switch", "master").Run(); err2 != nil {
			return fmt.Errorf("could not switch back to main/master")
		}
	}
	cmd := exec.Command("git", "worktree", "add", dir, branch)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	emitPath(dir)
	return nil
}

// openInEditor launches $EDITOR on file, wiring stdin/stdout/stderr to the
// terminal (via stderr so we don't pollute the stdout cd-path channel).
// Falls back to `vi` when $EDITOR is unset.
func openInEditor(file string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open %s in %s: %w", file, editor, err)
	}
	return nil
}
