package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
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

	changed, note, err := syncRoot(wt.Path)
	if err != nil {
		return err
	}
	if changed {
		if note != "" {
			pterm.Success.Printfln("synced %s (%s)", wt, note)
		} else {
			pterm.Success.Printfln("synced %s (updated)", wt)
		}
	} else {
		pterm.Info.Printfln("synced %s (no-op)", wt)
	}
	warnIfBroken()
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
		name    string
		changed bool
		note    string
		err     error
	}
	results := make([]result, len(wts))

	var wg sync.WaitGroup
	for i, wt := range wts {
		wg.Add(1)
		go func(i int, wt worktree) {
			defer wg.Done()
			defer bar.Increment()
			changed, note, err := syncRoot(wt.Path)
			results[i] = result{name: wt.String(), changed: changed, note: note, err: err}
		}(i, wt)
	}
	wg.Wait()

	rows := pterm.TableData{{"status", "plan", "note"}}
	failed := 0
	for _, r := range results {
		switch {
		case r.err != nil:
			rows = append(rows, []string{pterm.Red("err"), r.name, r.err.Error()})
			failed++
		case r.changed:
			rows = append(rows, []string{pterm.Green("updated"), r.name, r.note})
		default:
			rows = append(rows, []string{pterm.Gray("no-op"), r.name, ""})
		}
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()

	warnIfBroken()
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
// plan.toml exists. Returns (changed, note, err) where changed=true means
// the plan.toml was rewritten and note is a short human-readable summary of
// what differed.
func syncRoot(root string) (bool, string, error) {
	branch, err := currentBranch(root)
	if err != nil {
		return false, "", fmt.Errorf("branch: %w", err)
	}
	owner, repo, err := originOwnerRepo(root)
	if err != nil {
		return false, "", fmt.Errorf("remote: %w", err)
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
		return false, "", fmt.Errorf("read plan: %w", err)
	}
	beforeBytes, err := toml.Marshal(p)
	if err != nil {
		return false, "", fmt.Errorf("marshal before: %w", err)
	}
	// Deep copy for later diffing.
	var before plan
	if err := toml.Unmarshal(beforeBytes, &before); err != nil {
		return false, "", fmt.Errorf("clone plan: %w", err)
	}

	prInfo, err := prForBranch(owner, repo, branch)
	if err != nil {
		return false, "", fmt.Errorf("find pr: %w", err)
	}

	if prInfo != nil {
		log.Debug("found pr", log.Args("url", prInfo.URL, "state", prInfo.State))
		prData, closes, err := pr(prInfo.URL)
		if err != nil {
			return false, "", fmt.Errorf("fetch pr: %w", err)
		}
		// GitHub returns "unknown" for mergeable when it hasn't computed the
		// state yet (lazy computation). Treat that as "no new data" and keep
		// the previous value to avoid churn on every sync.
		if prData.Mergeable == "unknown" && p.PR.Mergeable != "" && p.PR.Mergeable != "unknown" {
			prData.Mergeable = p.PR.Mergeable
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

	afterBytes, err := toml.Marshal(p)
	if err != nil {
		return false, "", fmt.Errorf("marshal after: %w", err)
	}
	if bytes.Equal(beforeBytes, afterBytes) {
		return false, "", nil // no-op
	}
	if err := writePlan(p); err != nil {
		return false, "", fmt.Errorf("write plan: %w", err)
	}
	return true, diffPlans(before, p), nil
}

// diffPlans produces a short human-readable summary of what changed between
// two plans. Used for the sync summary "note" column.
func diffPlans(a, b plan) string {
	var parts []string

	// PR: appearance
	switch {
	case a.PR.URL == "" && b.PR.URL != "":
		parts = append(parts, "new pr")
	case a.PR.URL != "" && b.PR.URL == "":
		parts = append(parts, "pr removed")
	}
	// PR: state
	if a.PR.URL != "" && b.PR.URL != "" && a.PR.Mergeable != b.PR.Mergeable {
		parts = append(parts, fmt.Sprintf("pr: %s→%s", nonEmpty(a.PR.Mergeable), nonEmpty(b.PR.Mergeable)))
	}
	// PR: title
	if a.PR.URL != "" && b.PR.URL != "" && a.PR.Title != b.PR.Title {
		parts = append(parts, "pr title changed")
	}
	// PR: comments (count delta + body edits on matching entries)
	if d := len(b.PR.Comments) - len(a.PR.Comments); d != 0 {
		parts = append(parts, fmt.Sprintf("%+d comments", d))
	}
	// Detect edits: match by (thread, author, source); if body changed → edit.
	beforeBodies := make(map[string]string, len(a.PR.Comments))
	for _, c := range a.PR.Comments {
		beforeBodies[c.Thread+"|"+c.Author+"|"+c.Source] = c.Comment
	}
	edited := 0
	for _, c := range b.PR.Comments {
		if prev, ok := beforeBodies[c.Thread+"|"+c.Author+"|"+c.Source]; ok && prev != c.Comment {
			edited++
		}
	}
	if edited > 0 {
		parts = append(parts, fmt.Sprintf("%d comment edited", edited))
	}

	// Issues: count delta
	if d := len(b.Issues) - len(a.Issues); d != 0 {
		parts = append(parts, fmt.Sprintf("%+d issues", d))
	}
	// Issues: closed transitions (only for issues present in both)
	byURL := make(map[string]Issue, len(a.Issues))
	for _, i := range a.Issues {
		byURL[i.URL] = i
	}
	closedNow, openedNow := 0, 0
	for _, bi := range b.Issues {
		if ai, ok := byURL[bi.URL]; ok {
			if !ai.Closed && bi.Closed {
				closedNow++
			}
			if ai.Closed && !bi.Closed {
				openedNow++
			}
		}
	}
	if closedNow > 0 {
		parts = append(parts, fmt.Sprintf("%d issue closed", closedNow))
	}
	if openedNow > 0 {
		parts = append(parts, fmt.Sprintf("%d issue reopened", openedNow))
	}

	if len(parts) == 0 {
		return "updated" // fields differed but nothing we surface
	}
	return strings.Join(parts, ", ")
}

func nonEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func hasIssueURL(issues []Issue, url string) bool {
	for _, i := range issues {
		if i.URL == url {
			return true
		}
	}
	return false
}
