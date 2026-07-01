package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/pterm/pterm"
)

type rmCmd struct {
	Name string `arg:"positional" help:"worktree name (empty → pick; . → current)"`
}

type cleanCmd struct {
	DryRun bool `arg:"-d,--dry-run" help:"show what would be removed"`
}

// runRm dispatches on what was selected:
//   - worktree → git worktree remove + emit main path if cwd is now gone
//   - task    → mark as closed (mv to ~/w/t/closed/)
func runRm(c *rmCmd) error {
	// Empty name → unified picker (both types).
	if c.Name == "" {
		spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
		items, err := loadInventory(true, true)
		_ = spinner.Stop()
		if err != nil {
			return fmt.Errorf("rm: %w", err)
		}
		if len(items) == 0 {
			return fmt.Errorf("rm: nothing to remove")
		}
		it, err := pickInventory(items)
		if err != nil {
			return fmt.Errorf("rm: %w", err)
		}
		return processRm(it)
	}

	// Named or "." → worktree only.
	wt, err := selectWorktree(c.Name)
	if err != nil {
		return fmt.Errorf("rm: %w", err)
	}
	return processRm(inventoryItem{Worktree: &wt})
}

// processRm executes the removal for whichever kind of item was chosen.
func processRm(it inventoryItem) error {
	switch {
	case it.Task != nil:
		p, err := moveTask(*it.Task, statusClosed)
		if err != nil {
			return fmt.Errorf("task done: %w", err)
		}
		pterm.Success.Printfln("done: %s", p.Title)
		return nil

	case it.Worktree != nil:
		wt := *it.Worktree
		mainDir := mainWorktreePath(wt.Path)
		converted, err := convertPlanToTaskIfPending(wt)
		if err != nil {
			return fmt.Errorf("convert plan: %w", err)
		}
		if err := removeWorktree(wt); err != nil {
			// Roll back the conversion so we don't orphan a task copy.
			if converted != "" {
				_ = os.Remove(converted)
			}
			return fmt.Errorf("remove: %w", err)
		}
		if converted != "" {
			pterm.Success.Printfln("converted plan → %s", relPath(converted))
		}
		pterm.Success.Printfln("removed %s", wt)
		// If our cwd was the removed tree, emit main so the shell cds out.
		if cwd, err := os.Getwd(); err == nil {
			if _, err := os.Stat(cwd); err != nil && mainDir != "" {
				emitPath(mainDir)
			}
		}
		return nil
	}
	return fmt.Errorf("rm: unknown item type")
}

// runClean walks all worktrees, collects ones whose PR is merged or closed and
// whose tree is clean, previews them, and (unless --dry-run) confirms before
// actually removing.
func runClean(c *cleanCmd) error {
	wts, err := listWorktrees()
	if err != nil {
		return fmt.Errorf("clean: %w", err)
	}

	type candidate struct {
		wt    worktree
		label string // "merged" or "closed"
	}
	type check struct {
		wt    worktree
		label string // empty → skipped
		skip  string // reason (empty = candidate)
	}

	// Collect phase: parallel PR lookups with progress bar.
	bar, _ := pterm.DefaultProgressbar.WithTotal(len(wts)).WithTitle("checking PRs").Start()
	checks := make([]check, len(wts))
	var wg sync.WaitGroup
	for i, wt := range wts {
		wg.Add(1)
		go func(i int, wt worktree) {
			defer wg.Done()
			defer bar.Increment()
			if wt.Branch == "main" || wt.Branch == "master" {
				checks[i] = check{wt: wt, skip: "main/master"}
				return
			}
			if isDirty(wt.Path) {
				checks[i] = check{wt: wt, skip: "dirty"}
				return
			}
			owner, repo, err := originOwnerRepo(wt.Path)
			if err != nil {
				checks[i] = check{wt: wt, skip: "no origin"}
				return
			}
			info, err := prForBranch(owner, repo, wt.Branch)
			if err != nil {
				checks[i] = check{wt: wt, skip: fmt.Sprintf("pr lookup: %v", err)}
				return
			}
			if info == nil {
				checks[i] = check{wt: wt, skip: "no PR"}
				return
			}
			if info.State != "MERGED" && info.State != "CLOSED" {
				checks[i] = check{wt: wt, skip: "PR " + strings.ToLower(info.State)}
				return
			}
			checks[i] = check{wt: wt, label: strings.ToLower(info.State)}
		}(i, wt)
	}
	wg.Wait()
	_, _ = bar.Stop()

	// Surface skips of note; collect candidates.
	var candidates []candidate
	for _, ch := range checks {
		switch {
		case ch.skip == "dirty":
			pterm.Warning.Printfln("skip | %s:%s dirty working tree", ch.wt.Repo, ch.wt.Branch)
		case strings.HasPrefix(ch.skip, "pr lookup"):
			pterm.Warning.Printfln("skip | %s:%s %s", ch.wt.Repo, ch.wt.Branch, ch.skip)
		case ch.skip != "":
			log.Debug("skip", log.Args("wt", ch.wt.Path, "reason", ch.skip))
		default:
			candidates = append(candidates, candidate{wt: ch.wt, label: ch.label})
		}
	}

	if len(candidates) == 0 {
		pterm.Info.Println("nothing to clean")
		return nil
	}

	// Preview phase: list what will be removed.
	for _, cand := range candidates {
		pterm.Info.Printfln("would rm | %s:%s %s",
			cand.wt.Repo, cand.wt.Branch, cand.label)
	}
	if c.DryRun {
		return nil
	}

	// Confirm phase.
	if !confirm(fmt.Sprintf("remove %d worktrees?", len(candidates))) {
		return fmt.Errorf("clean cancelled")
	}

	// Remove phase.
	for _, cand := range candidates {
		if err := removeWorktree(cand.wt); err != nil {
			pterm.Warning.Printfln("FAIL %s: %v", cand.wt, err)
			continue
		}
		pterm.Success.Printfln("rm | %s:%s %s",
			cand.wt.Repo, cand.wt.Branch, cand.label)
	}
	return nil
}

// selectWorktree resolves a target worktree from a name (empty → picker,
// "." → current cwd's worktree, otherwise a rel path or branch slug).
func selectWorktree(name string) (worktree, error) {
	switch name {
	case "":
		return pickWorktree()
	case ".":
		return currentWorktree()
	}
	return resolveWorktree(name)
}

// currentWorktree returns the worktree that contains the current cwd.
func currentWorktree() (worktree, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return worktree{}, err
	}
	if !strings.HasPrefix(cwd+"/", defaultWorkDir+"/") {
		return worktree{}, fmt.Errorf("not inside a worktree under %s", defaultWorkDir)
	}
	wts, err := listWorktrees()
	if err != nil {
		return worktree{}, err
	}
	for _, w := range wts {
		if cwd == w.Path || strings.HasPrefix(cwd+"/", w.Path+"/") {
			return w, nil
		}
	}
	return worktree{}, fmt.Errorf("cwd is not a known worktree")
}

// resolveWorktree finds a worktree by name — either "<repo>/<slug>" (rel to
// ~/w) or a bare branch slug (searched across repos, current repo first).
func resolveWorktree(name string) (worktree, error) {
	wts, err := listWorktrees()
	if err != nil {
		return worktree{}, err
	}
	target := path.Join(defaultWorkDir, name)
	for _, w := range wts {
		if w.Path == target {
			return w, nil
		}
	}
	slug := branchSlug(name)
	// prefer current repo
	if repo, err := currentRepoName(); err == nil {
		for _, w := range wts {
			if w.Repo == repo && (branchSlug(w.Branch) == slug || path.Base(w.Path) == slug) {
				return w, nil
			}
		}
	}
	// fall back to any repo
	for _, w := range wts {
		if branchSlug(w.Branch) == slug || path.Base(w.Path) == slug {
			return w, nil
		}
	}
	return worktree{}, fmt.Errorf("no worktree matching %q", name)
}

// convertPlanToTaskIfPending checks whether the worktree's plan.toml has any
// tasks[] entries; if so, it writes a copy into ~/w/t/<status>/<N>.toml
// preserving the plan's current status. Branches are the highest form of
// work, tasks are underdeveloped follow-up, so this is called a conversion
// (not a promotion — that word is reserved for the inverse direction). If the plan
// is absent, unparseable, or has no tasks, returns "" and nil (nothing to
// do). The original plan.toml stays put; the caller is expected to remove
// the worktree next, which deletes the original along with the rest of the
// tree.
//
// A worktree with status=closed but tasks[] still populated is treated as an
// anomaly: we warn and, on confirmation, land the converted task in `working`
// so it surfaces in `work list` and doesn't orphan.
func convertPlanToTaskIfPending(wt worktree) (string, error) {
	planPath := path.Join(wt.Path, planFileName)
	p, err := readPlan(planPath)
	if err != nil {
		// Missing or broken — nothing to convert.
		return "", nil
	}
	if len(p.Tasks) == 0 {
		return "", nil
	}
	if p.Status == statusClosed {
		pterm.Warning.Printfln("worktree %s is closed but has %d open task(s)",
			relPath(wt.Path), len(p.Tasks))
		if !confirm("convert to a working task instead?") {
			return "", fmt.Errorf("convert cancelled")
		}
		p.Status = statusWorking
	}
	if p.Status == "" {
		p.Status = statusOpen
	}
	n, err := nextTaskNum()
	if err != nil {
		return "", fmt.Errorf("alloc task num: %w", err)
	}
	dir := taskDir(p.Status)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	newPath := path.Join(dir, fmt.Sprintf("%d.toml", n))
	p.Path = newPath
	if err := writePlan(p); err != nil {
		return "", fmt.Errorf("write converted task: %w", err)
	}
	return newPath, nil
}

// removeWorktree runs `git worktree remove` and cleans up an empty repo parent.
func removeWorktree(wt worktree) error {
	cmd := exec.Command("git", "-C", wt.Path, "worktree", "remove", wt.Path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove: %w", err)
	}
	_ = os.Remove(path.Dir(wt.Path)) // ignore — non-empty is fine
	return nil
}

// mainWorktreePath returns the main worktree of the repo containing dir, or "" on failure.
// Uses `--porcelain` so paths with spaces are handled correctly (the first
// `worktree <path>` block in the output is always the main tree).
func mainWorktreePath(dir string) string {
	out, err := exec.Command("git", "-C", dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			return p
		}
	}
	return ""
}

// isDirty reports whether the working tree at dir has uncommitted changes.
func isDirty(dir string) bool {
	if err := exec.Command("git", "-C", dir, "diff", "--quiet").Run(); err != nil {
		return true
	}
	if err := exec.Command("git", "-C", dir, "diff", "--cached", "--quiet").Run(); err != nil {
		return true
	}
	return false
}

// relPath returns dir relative to ~/w (falls back to dir if it's outside).
func relPath(dir string) string {
	if strings.HasPrefix(dir, defaultWorkDir+"/") {
		return strings.TrimPrefix(dir, defaultWorkDir+"/")
	}
	return dir
}
