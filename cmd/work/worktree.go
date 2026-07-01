package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

const prevWorktreeFile = ".previous"

// worktree describes a directory under ~/w/<repo>/<branch>.
type worktree struct {
	Path   string    // absolute path
	Repo   string    // <repo> component of ~/w/<repo>/<branch>
	Branch string    // resolved via git symbolic-ref, falls back to dir name
	Mtime  time.Time // last commit time (git log -1 --format=%ct), fallback to fs mtime
}

// String returns the user-facing "<repo>:<branch>" form. Never expose paths.
func (w worktree) String() string { return w.Repo + ":" + w.Branch }

// listWorktrees walks ~/w/*/* and returns discovered worktrees, sorted by
// mtime ascending (newest last). Enrichment (branch, mtime) runs in parallel
// since each dir requires two git execs.
func listWorktrees() ([]worktree, error) {
	matches, err := filepath.Glob(path.Join(defaultWorkDir, "*", "*"))
	if err != nil {
		return nil, fmt.Errorf("glob worktrees: %w", err)
	}

	// First pass: filter to valid candidate dirs (cheap, sync).
	type candidate struct {
		dir  string
		info os.FileInfo
	}
	var candidates []candidate
	for _, dir := range matches {
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			continue
		}
		if strings.HasPrefix(dir, defaultTaskDir+"/") || dir == defaultTaskDir {
			continue
		}
		rel := strings.TrimPrefix(dir, defaultWorkDir+"/")
		if strings.Count(rel, "/") < 1 {
			continue
		}
		candidates = append(candidates, candidate{dir, fi})
	}

	// Second pass: fan out git execs per candidate.
	wts := make([]worktree, len(candidates))
	var wg sync.WaitGroup
	for i, c := range candidates {
		wg.Add(1)
		go func(i int, c candidate) {
			defer wg.Done()
			rel := strings.TrimPrefix(c.dir, defaultWorkDir+"/")
			parts := strings.SplitN(rel, "/", 2)
			wt := worktree{Path: c.dir, Repo: parts[0], Branch: parts[1]}
			if b, err := exec.Command("git", "-C", c.dir, "symbolic-ref", "--short", "HEAD").Output(); err == nil {
				wt.Branch = strings.TrimSpace(string(b))
			}
			if out, err := exec.Command("git", "-C", c.dir, "log", "-1", "--format=%ct").Output(); err == nil {
				if secs, perr := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); perr == nil {
					wt.Mtime = time.Unix(secs, 0)
				}
			}
			if wt.Mtime.IsZero() {
				wt.Mtime = c.info.ModTime()
			}
			wts[i] = wt
		}(i, c)
	}
	wg.Wait()

	sort.Slice(wts, func(i, j int) bool { return wts[i].Mtime.Before(wts[j].Mtime) })
	return wts, nil
}

// pickInventory shows an interactive select over the given items (using
// pterm's built-in filter). Returns the chosen item, or an error if cancelled.
func pickInventory(items []inventoryItem) (inventoryItem, error) {
	if len(items) == 0 {
		return inventoryItem{}, fmt.Errorf("nothing to pick")
	}
	labels := formatLabels(items)
	byLabel := make(map[string]inventoryItem, len(items))
	for i, it := range items {
		byLabel[labels[i]] = it
	}
	sel, err := pterm.DefaultInteractiveSelect.
		WithOptions(labels).
		WithFilter(true).
		WithMaxHeight(20).
		Show()
	if err != nil {
		return inventoryItem{}, fmt.Errorf("pick: %w", err)
	}
	it, ok := byLabel[sel]
	if !ok {
		return inventoryItem{}, fmt.Errorf("pick: no match for %q", sel)
	}
	return it, nil
}

// pickWorktree is a convenience wrapper: shows worktrees only.
func pickWorktree() (worktree, error) {
	spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
	items, err := loadInventory(true, false)
	_ = spinner.Stop()
	if err != nil {
		return worktree{}, err
	}
	if len(items) == 0 {
		return worktree{}, fmt.Errorf("no worktrees under %s", defaultWorkDir)
	}
	it, err := pickInventory(items)
	if err != nil {
		return worktree{}, err
	}
	if it.Worktree == nil {
		return worktree{}, fmt.Errorf("pick: expected a worktree")
	}
	return *it.Worktree, nil
}

// timeAgo renders a duration like the bash version did: 5m, 2h, 3d, 1w, 2mo.
func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}

// savePrevious records dir as the "previous" worktree for `work prev`.
func savePrevious(dir string) error {
	return os.WriteFile(path.Join(defaultWorkDir, prevWorktreeFile), []byte(dir), planFileMode)
}

// readPrevious returns the previously-emitted path.
func readPrevious() (string, error) {
	data, err := os.ReadFile(path.Join(defaultWorkDir, prevWorktreeFile))
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(data))
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("previous dir gone: %w", err)
	}
	return p, nil
}

// emitPath writes target to the real stdout (captured at boot into realStdout)
// so the shell wrapper's $() picks it up for cd. All other output stays on the
// terminal via the fd-1 → fd-2 dup2 done in stdout.go's init.
func emitPath(target string) {
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(cwd); err == nil && cwd != target {
			_ = savePrevious(cwd)
		}
	}
	if realStdout == nil || realStdoutIsTerminal() {
		fmt.Fprintln(os.Stderr, "shim not installed; nothing is capturing stdout to cd for you.")
		fmt.Fprintln(os.Stderr, "install with: work install  (path emitted below)")
	}
	if realStdout != nil {
		fmt.Fprintln(realStdout, target)
	} else {
		fmt.Println(target)
	}
}

// realStdoutIsTerminal reports whether the real (pre-dup2) stdout is attached
// to a TTY. If it is, no wrapper is capturing it.
func realStdoutIsTerminal() bool {
	if realStdout == nil {
		return false
	}
	fi, err := realStdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// currentRepoRoot returns the repo name for the current worktree by
// inspecting git's main worktree path and taking its basename.
func currentRepoName() (string, error) {
	root, err := repoRoot(".")
	if err != nil {
		return "", err
	}
	// If we're inside ~/w/<repo>/<branch>, use <repo>.
	if strings.HasPrefix(root, defaultWorkDir+"/") {
		rel := strings.TrimPrefix(root, defaultWorkDir+"/")
		parts := strings.SplitN(rel, "/", 2)
		return parts[0], nil
	}
	// Otherwise use the basename of the toplevel.
	return path.Base(root), nil
}

// findWorktree returns the worktree path matching (repo, branchSlug) or "" if none.
func findWorktree(branchSlug string) string {
	// prefer current repo
	if repo, err := currentRepoName(); err == nil {
		dir := path.Join(defaultWorkDir, repo, branchSlug)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	// scan across all repos
	matches, _ := filepath.Glob(path.Join(defaultWorkDir, "*", branchSlug))
	for _, m := range matches {
		if fi, err := os.Stat(m); err == nil && fi.IsDir() {
			return m
		}
	}
	return ""
}
