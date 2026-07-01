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
	Arg string `arg:"positional,required" help:"'.' for worktree from current branch; \"title with spaces\" for a chore"`
}

// runNew dispatches on the shape of Arg:
//   - "."                → worktree from the current branch
//   - contains a space   → chore (title stored in ~/w/x/open/N.toml), opens $EDITOR
//   - anything else      → error (worktree-from-branch is no longer supported)
func runNew(c *newCmd) error {
	switch {
	case c.Arg == ".":
		return newFromCurrent()
	case strings.Contains(c.Arg, " "):
		p, err := newChore(c.Arg)
		if err != nil {
			return err
		}
		pterm.Success.Printfln("chore #%s: %s",
			path.Base(strings.TrimSuffix(p.Path, ".toml")), p.Title)
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
