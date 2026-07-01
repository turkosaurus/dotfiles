package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/pterm/pterm"
)

func runSync(args *syncCmd) error {
	if args.All {
		return runSyncAll()
	}

	// Single-sync uses the managed worktree containing cwd (~/w/<repo>/<branch>),
	// not `git rev-parse --show-toplevel` (which would land at an arbitrary git
	// root outside ~/w).
	wt, err := currentWorktree()
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	planPath := path.Join(wt.Path, planFileName)
	_, statErr := os.Stat(planPath)
	switch {
	case os.IsNotExist(statErr):
		if !confirm(fmt.Sprintf("no plan.toml in %s. Create one?", wt)) {
			return fmt.Errorf("sync cancelled")
		}
		if err := seedPlan(planPath, wt.Branch); err != nil {
			return fmt.Errorf("seed plan: %w", err)
		}
		pterm.Success.Printfln("seeded %s", wt)
	case statErr != nil:
		return fmt.Errorf("stat plan: %w", statErr)
	default:
		// Exists — confirm it parses.
		if _, err := readPlan(planPath); err != nil {
			skip, herr := handleBrokenPlan(planPath, wt.String(), err)
			if herr != nil {
				return herr
			}
			if skip {
				return nil
			}
		}
	}

	if err := syncRoot(wt.Path); err != nil {
		return err
	}
	pterm.Success.Printfln("synced %s", wt)
	return nil
}

// handleBrokenPlan prompts [e]dit / [s]kip / [q]uit when plan.toml at
// planPath fails to parse. Returns (skip, err): skip=true means the caller
// should return nil (don't touch this plan); err != nil is a fatal abort.
func handleBrokenPlan(planPath, displayName string, parseErr error) (bool, error) {
	rdr := bufio.NewReader(os.Stdin)
	for {
		pterm.Error.Printfln("plan.toml for %s won't parse: %v", displayName, parseErr)
		fmt.Fprint(os.Stderr, "[e]dit / [s]kip / [q]uit: ")
		line, err := rdr.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("read prompt: %w", err)
		}
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "e":
			if err := openInEditor(planPath); err != nil {
				return false, err
			}
			if _, err := readPlan(planPath); err == nil {
				return false, nil // fixed
			} else {
				parseErr = err // loop with fresh error
			}
		case "s":
			return true, nil
		case "q":
			return false, fmt.Errorf("aborted: %w", parseErr)
		default:
			pterm.Warning.Println("please answer e, s, or q")
		}
	}
}

func runSyncAll() error {
	// Scan every worktree, not just those with an existing plan.toml.
	wts, err := listWorktrees()
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	if len(wts) == 0 {
		pterm.Info.Printfln("no worktrees under %s", defaultWorkDir)
		return nil
	}

	// Bucket: which need seeding?
	var missing []worktree
	for _, wt := range wts {
		if _, err := os.Stat(path.Join(wt.Path, planFileName)); os.IsNotExist(err) {
			missing = append(missing, wt)
		}
	}

	if len(missing) > 0 {
		for _, wt := range missing {
			pterm.Info.Printfln("no plan.toml in %s", wt)
		}
		if !confirm(fmt.Sprintf("Create %d plan.toml files?", len(missing))) {
			return fmt.Errorf("sync cancelled")
		}
		for _, wt := range missing {
			if err := seedPlan(path.Join(wt.Path, planFileName), wt.Branch); err != nil {
				pterm.Warning.Printfln("seed %s: %v", wt, err)
			}
		}
	}

	// Parallel sync with progress bar.
	bar, _ := pterm.DefaultProgressbar.WithTotal(len(wts)).WithTitle("syncing").Start()
	defer func() { _, _ = bar.Stop() }()

	type result struct {
		name string
		err  error
	}
	results := make([]result, len(wts))

	var wg sync.WaitGroup
	for i, wt := range wts {
		wg.Add(1)
		go func(i int, wt worktree) {
			defer wg.Done()
			defer bar.Increment()
			results[i] = result{name: wt.String(), err: syncRoot(wt.Path)}
		}(i, wt)
	}
	wg.Wait()

	rows := pterm.TableData{{"plan", "status", "note"}}
	failed := 0
	for _, r := range results {
		if r.err != nil {
			rows = append(rows, []string{r.name, pterm.Red("FAIL"), r.err.Error()})
			failed++
			continue
		}
		rows = append(rows, []string{r.name, pterm.Green("ok"), ""})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()

	if failed > 0 {
		return fmt.Errorf("%d of %d plans failed", failed, len(wts))
	}
	return nil
}

// seedPlan creates a minimal plan.toml at planPath with title = branch.
func seedPlan(planPath, branch string) error {
	p := defaultPlan(branch)
	p.Path = planPath
	return writePlan(p)
}

// syncRoot performs the sync for a single worktree root. Caller must ensure
// plan.toml exists.
func syncRoot(root string) error {
	branch, err := currentBranch(root)
	if err != nil {
		return fmt.Errorf("branch: %w", err)
	}
	owner, repo, err := originOwnerRepo(root)
	if err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	log.Debug("repo state", log.Args(
		"owner", owner,
		"repo", repo,
		"branch", branch,
		"root", root,
	))

	planPath := path.Join(root, planFileName)
	p, err := readPlan(planPath)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}

	prInfo, err := prForBranch(owner, repo, branch)
	if err != nil {
		return fmt.Errorf("find pr: %w", err)
	}

	if prInfo != nil {
		log.Debug("found pr", log.Args("url", prInfo.URL, "state", prInfo.State))
		prData, closes, err := pr(prInfo.URL)
		if err != nil {
			return fmt.Errorf("fetch pr: %w", err)
		}
		p.PR = prData
		for _, ref := range closes {
			if !hasIssueURL(p.Issues, ref.URL) {
				p.Issues = append(p.Issues, Issue{URL: ref.URL})
			}
		}
	} else {
		log.Debug("no PR for branch", log.Args("branch", branch))
	}

	for i := range p.Issues {
		if p.Issues[i].URL == "" {
			continue
		}
		fresh, _, err := issue(p.Issues[i].URL)
		if err != nil {
			pterm.Warning.Printfln("issue %s: %v", p.Issues[i].URL, err)
			continue
		}
		p.Issues[i] = fresh
	}

	if err := writePlan(p); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
}

func hasIssueURL(issues []Issue, url string) bool {
	for _, i := range issues {
		if i.URL == url {
			return true
		}
	}
	return false
}
