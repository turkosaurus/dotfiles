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
		return runSyncAll(args.DryRun)
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

	changed, note, err := syncRoot(wt.Path, args.DryRun)
	reportWorktreeSync(wt.String(), changed, note, err, args.DryRun)

	if serr := runSprintSync(args.DryRun); serr != nil {
		return serr
	}
	warnIfBroken()
	if err != nil {
		// reportWorktreeSync already printed the error line — return the
		// silent sentinel so main.dispatch doesn't reprint it.
		return errPrinted
	}
	return nil
}

// reportWorktreeSync prints a single line describing what happened to one
// worktree's plan.toml, using the same shape as sprint reconcile lines:
//   {would update|updated} <name> (<detail>)
// where detail is "no-op" when nothing changed, the diffPlans note when
// something changed, or an error's message. The verb comes from the
// dryRun flag; the phrase inside parens comes from the actual outcome.
func reportWorktreeSync(name string, changed bool, note string, err error, dryRun bool) {
	verb := "updated"
	if dryRun {
		verb = "would update"
	}
	switch {
	case err != nil:
		pterm.Error.Printfln("%s %s (error: %v)", verb, name, err)
	case !changed:
		pterm.Info.Printfln("%s %s (no-op)", verb, name)
	case dryRun:
		pterm.Info.Printfln("%s %s (%s)", verb, name, orNote(note))
	default:
		pterm.Success.Printfln("%s %s (%s)", verb, name, orNote(note))
	}
}

// orNote returns note when non-empty, else "updated" as a fallback detail
// (rare: change was structural but diffPlans couldn't summarize).
func orNote(note string) string {
	if note == "" {
		return "updated"
	}
	return note
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

func runSyncAll(dryRun bool) error {
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

	// Two-bar layout via MultiPrinter so the worktree fetch and the sprint
	// fetch progress side-by-side without stepping on each other. Sprint is
	// a 2-tick bar: 1/2 after fetch completes (network), 2/2 after
	// reconcile completes (disk writes). All per-item output is buffered
	// and printed after both bars stop.
	multi := pterm.DefaultMultiPrinter
	if _, err := multi.Start(); err != nil {
		log.Debug("multi.Start", log.Args("err", err))
	}

	// Sprint spinner first (top of the stacked MultiPrinter area), then the
	// worktree progress bar below. WithRemoveWhenDone drops the bar frame
	// on Stop so we can print an OK line in its place.
	sprintSpinner, _ := pterm.DefaultSpinner.
		WithText("syncing sprint").
		WithWriter(multi.NewWriter()).
		Start()
	wtWriter := multi.NewWriter()
	wtBar, _ := pterm.DefaultProgressbar.
		WithTotal(len(wts)).
		WithTitle("worktrees").
		WithWriter(wtWriter).
		WithRemoveWhenDone(true).
		Start()

	sprintCh := make(chan sprintFetchResult, 1)
	go func() { sprintCh <- fetchSprint(nil) }()

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
			defer wtBar.Increment()
			changed, note, err := syncRoot(wt.Path, dryRun)
			results[i] = result{name: wt.String(), changed: changed, note: note, err: err}
		}(i, wt)
	}
	wg.Wait()
	if _, err := wtBar.Stop(); err != nil {
		log.Debug("wtBar.Stop", log.Args("err", err))
	}
	// Bar removed itself (WithRemoveWhenDone); print an OK line into the
	// same MultiPrinter area so the "worktrees: N synced" line appears in
	// the row the bar previously occupied.
	pterm.Success.WithWriter(wtWriter).Printfln("worktrees: %d synced", len(wts))

	// Plan-only pass — no side effects. Actions are queued for later.
	sprintRes := <-sprintCh
	sprintOut, sprintActions, serr := planSprint(sprintRes)
	switch {
	case sprintRes.err != nil:
		sprintSpinner.Fail("sprint fetch failed")
	case serr != nil:
		sprintSpinner.Fail("sprint plan failed")
	case sprintRes.disabled:
		if sErr := sprintSpinner.Stop(); sErr != nil {
			log.Debug("sprintSpinner.Stop", log.Args("err", sErr))
		}
	default:
		sprintSpinner.Success(fmt.Sprintf("sprint: %d project items", len(sprintRes.items)))
	}
	if _, err := multi.Stop(); err != nil {
		log.Debug("multi.Stop", log.Args("err", err))
	}

	// Now bars are down; flush all buffered output.
	failed := 0
	for _, r := range results {
		reportWorktreeSync(r.name, r.changed, r.note, r.err, dryRun)
		if r.err != nil {
			failed++
		}
	}
	sprintOut.flush()
	if serr != nil {
		return serr
	}
	// Confirm + apply the sprint actions (skipped in dry-run). Silent
	// stamp writes are auto-applied without contributing to the confirm
	// count — the user only decides about real status changes / creates.
	sprintFailed := 0
	if !dryRun && !sprintRes.disabled {
		loud := countUserFacingActions(sprintActions)
		switch {
		case loud == 0:
			pterm.Info.Println("sprint: nothing to apply")
		case !confirm(fmt.Sprintf("apply %d sprint change(s)?", loud)):
			pterm.Info.Println("sprint sync cancelled")
		default:
			preApply := len(sprintOut.lines)
			sprintFailed = applySprint(sprintActions, &sprintOut)
			for _, l := range sprintOut.lines[preApply:] {
				fmt.Print(l)
			}
			if sprintFailed == 0 {
				pterm.Success.Printfln("applied %d sprint change(s)", loud)
			}
		}
	}
	warnIfBroken()
	switch {
	case failed > 0 && sprintFailed > 0:
		return fmt.Errorf("%d worktree(s) + %d sprint apply(s) failed", failed, sprintFailed)
	case failed > 0:
		return fmt.Errorf("%d of %d plans failed", failed, len(wts))
	case sprintFailed > 0:
		return fmt.Errorf("sprint: %d apply(s) failed", sprintFailed)
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
// the plan.toml would-be / was rewritten and note summarizes what differed.
//
// dryRun=true suppresses the actual write. `changed` still reflects whether
// content would differ so the caller can label the line "would update".
func syncRoot(root string, dryRun bool) (bool, string, error) {
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
		prevMergeable := firstPR(p).Mergeable
		if prData.Mergeable == "unknown" && prevMergeable != "" && prevMergeable != "unknown" {
			prData.Mergeable = prevMergeable
		}
		p.PRs = upsertPR(p.PRs, prData)
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
	if dryRun {
		return true, diffPlans(before, p), nil
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

	aPR, bPR := firstPR(a), firstPR(b)

	// PR: appearance
	switch {
	case aPR.URL == "" && bPR.URL != "":
		parts = append(parts, "new pr")
	case aPR.URL != "" && bPR.URL == "":
		parts = append(parts, "pr removed")
	}
	// PR: state
	if aPR.URL != "" && bPR.URL != "" && aPR.Mergeable != bPR.Mergeable {
		parts = append(parts, fmt.Sprintf("pr: %s→%s", nonEmpty(aPR.Mergeable), nonEmpty(bPR.Mergeable)))
	}
	// PR: title
	if aPR.URL != "" && bPR.URL != "" && aPR.Title != bPR.Title {
		parts = append(parts, "pr title changed")
	}
	// PR: comments (count delta + body edits on matching entries)
	if d := len(bPR.Comments) - len(aPR.Comments); d != 0 {
		parts = append(parts, fmt.Sprintf("%+d comments", d))
	}
	// Detect edits: match by (thread, author, source); if body changed → edit.
	beforeBodies := make(map[string]string, len(aPR.Comments))
	for _, c := range aPR.Comments {
		beforeBodies[c.Thread+"|"+c.Author+"|"+c.Source] = c.Comment
	}
	edited := 0
	for _, c := range bPR.Comments {
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
