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

// runSprintOnly is the fallback for `work sync` invoked from a directory
// outside any managed worktree. It skips the worktree sync leg (there's
// nothing to sync) and just runs the sprint fetch + reconcile + task
// location reconcile. Useful when you're in ~/w/t/ hand-editing tasks
// and want a resync without having to cd back to a branch.
func runSprintOnly(dryRun bool) error {
	spinner, _ := pterm.DefaultSpinner.WithText("syncing sprint").Start()
	sprintRes := fetchSprint(nil)
	sprintOut, sprintActions, serr := planSprint(sprintRes)
	switch {
	case sprintRes.err != nil:
		spinner.Fail("sprint fetch failed")
	case serr != nil:
		spinner.Fail("sprint plan failed")
	case sprintRes.disabled:
		if sErr := spinner.Stop(); sErr != nil {
			log.Debug("spinner.Stop", log.Args("err", sErr))
		}
	default:
		spinner.Success(fmt.Sprintf("sprint: %d project items fetched", len(sprintRes.items)))
	}
	sprintOut.flushLines()
	if serr != nil {
		return serr
	}
	sprintFailed := 0
	if !sprintRes.disabled {
		loud := countUserFacingActions(sprintActions)
		if loud > 0 {
			sprintOut.renderTable()
		}
		if !dryRun {
			switch {
			case loud == 0:
				log.Debug("sprint: nothing to apply")
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
	}
	if !dryRun {
		reconcileTaskLocations()
	}
	warnIfBroken()
	if sprintFailed > 0 {
		return fmt.Errorf("sprint: %d apply(s) failed", sprintFailed)
	}
	return nil
}

func runSync(args *syncCmd) error {
	if args.All {
		return runSyncAll(args.DryRun)
	}

	// Single-sync uses the managed worktree containing cwd (~/w/<repo>/<branch>),
	// not `git rev-parse --show-toplevel` (which would land at an arbitrary
	// git root outside ~/w). When cwd isn't inside a managed worktree we
	// still want sprint sync + task reconcile to run, so degrade to
	// "sprint-only" instead of erroring out.
	wt, wtErr := currentWorktree()
	if wtErr != nil {
		log.Debug("not inside a worktree; skipping worktree sync", log.Args("err", wtErr))
		return runSprintOnly(args.DryRun)
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

	// Both the worktree sync and the sprint fetch share one MultiPrinter
	// so their spinners render in distinct rows (no cursor collisions).
	// Sprint fetch runs concurrently with syncRoot; reconcile still waits
	// for syncRoot + auto-close so trackedPlans reads a consistent view.
	multi := pterm.DefaultMultiPrinter
	if _, err := multi.Start(); err != nil {
		log.Debug("multi.Start", log.Args("err", err))
	}
	sprintSpinner, _ := pterm.DefaultSpinner.
		WithText("syncing sprint").
		WithWriter(multi.NewWriter()).
		Start()
	wtSpinner, _ := pterm.DefaultSpinner.
		WithText(fmt.Sprintf("syncing %s", wt)).
		WithWriter(multi.NewWriter()).
		Start()

	sprintCh := make(chan sprintFetchResult, 1)
	go func() { sprintCh <- fetchSprint(nil) }()

	res := syncRoot(wt.Path, args.DryRun)
	verb := "updated"
	if args.DryRun {
		verb = "would update"
	}
	switch {
	case res.err != nil:
		wtSpinner.Fail(fmt.Sprintf("error %s: %v", wt, res.err))
	case !res.changed:
		wtSpinner.Success(fmt.Sprintf("%s: PR details fetched (no-op)", wt))
	default:
		wtSpinner.Success(fmt.Sprintf("%s %s (%s)", verb, wt, orNote(res.note)))
	}

	// Auto-close the worktree when its PR reached a terminal state.
	// Skipped in dry-run so `-d` is genuinely read-only. Done before
	// planSprint so trackedPlans sees the converted task (if any).
	if res.autoClose && !args.DryRun {
		if err := closeMergedWorktree(wt, res.closeReason); err != nil {
			pterm.Warning.Printfln("auto-close %s: %v", wt, err)
		}
	}

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
		sprintSpinner.Success(fmt.Sprintf("sprint: %d project items fetched", len(sprintRes.items)))
	}
	if _, err := multi.Stop(); err != nil {
		log.Debug("multi.Stop", log.Args("err", err))
	}

	sprintOut.flushLines()
	if serr != nil {
		return serr
	}

	sprintFailed := 0
	if !sprintRes.disabled {
		loud := countUserFacingActions(sprintActions)
		if loud > 0 {
			sprintOut.renderTable()
		}
		if !args.DryRun {
			switch {
			case loud == 0:
				log.Debug("sprint: nothing to apply")
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
	}

	if !args.DryRun {
		reconcileTaskLocations()
	}
	warnIfBroken()
	switch {
	case res.err != nil && sprintFailed > 0:
		return fmt.Errorf("worktree error + %d sprint apply(s) failed", sprintFailed)
	case res.err != nil:
		return errPrinted
	case sprintFailed > 0:
		return fmt.Errorf("sprint: %d apply(s) failed", sprintFailed)
	}
	return nil
}

// reportWorktreeSync prints a single line describing what happened to one
// worktree's plan.toml — but only when there's something worth surfacing.
// No-ops go to the debug log so `-v` still shows them; the default view
// stays focused on real (or would-be) mutations and errors.
//
// Shape matches sprint reconcile lines:
//   {would update|updated} <name> (<detail>)
func reportWorktreeSync(name string, changed bool, note string, err error, dryRun bool) {
	verb := "updated"
	if dryRun {
		verb = "would update"
	}
	switch {
	case err != nil:
		pterm.Error.Printfln("%s %s (error: %v)", verb, name, err)
	case !changed:
		log.Debug("worktree sync no-op", log.Args("name", name))
	default:
		// Would-be and real mutations both use Success — the verb tells
		// the user which. INFO is reserved for informational output that
		// isn't a mutation.
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
		wt          worktree
		name        string
		changed     bool
		note        string
		autoClose   bool
		closeReason string
		err         error
	}
	results := make([]result, len(wts))
	var wg sync.WaitGroup
	for i, wt := range wts {
		wg.Add(1)
		go func(i int, wt worktree) {
			defer wg.Done()
			defer wtBar.Increment()
			out := syncRoot(wt.Path, dryRun)
			results[i] = result{
				wt:          wt,
				name:        wt.String(),
				changed:     out.changed,
				note:        out.note,
				autoClose:   out.autoClose,
				closeReason: out.closeReason,
				err:         out.err,
			}
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
		sprintSpinner.Success(fmt.Sprintf("sprint: %d project items fetched", len(sprintRes.items)))
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
	// Sequential auto-close pass: git worktree operations don't parallel
	// well and the tool's cwd may live in one of the removed dirs.
	if !dryRun {
		for _, r := range results {
			if !r.autoClose {
				continue
			}
			if err := closeMergedWorktree(r.wt, r.closeReason); err != nil {
				pterm.Warning.Printfln("auto-close %s: %v", r.wt, err)
			}
		}
	}
	sprintOut.flushLines()
	if serr != nil {
		return serr
	}
	// Confirm + apply the sprint actions (skipped in dry-run). Silent
	// stamp writes are auto-applied without contributing to the confirm
	// count — the user only decides about real status changes / creates.
	// Table only shown when there's something to apply.
	sprintFailed := 0
	if !sprintRes.disabled {
		loud := countUserFacingActions(sprintActions)
		if loud > 0 {
			sprintOut.renderTable()
		}
		if !dryRun {
			switch {
			case loud == 0:
				log.Debug("sprint: nothing to apply")
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
	}
	if !dryRun {
		reconcileTaskLocations()
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

// syncOutcome carries syncRoot's result. autoClose is set when the fresh
// fetch shows a PR that just reached a terminal state (MERGED or CLOSED)
// and the local plan wasn't already closed. closeReason carries the
// terminal state ("merged" / "closed") for the cleanup log line.
type syncOutcome struct {
	changed     bool
	note        string
	autoClose   bool
	closeReason string
	err         error
}

// syncRoot performs the sync for a single worktree root. Caller must ensure
// plan.toml exists. `changed` reflects whether content would differ so the
// caller can label the line "would update". `autoClose` reports the merged-
// PR transition; the caller handles the worktree removal + task conversion.
//
// dryRun=true suppresses the actual write. autoClose still reports true so
// the caller can preview the intended cleanup.
func syncRoot(root string, dryRun bool) syncOutcome {
	branch, err := currentBranch(root)
	if err != nil {
		return syncOutcome{err: fmt.Errorf("branch: %w", err)}
	}
	owner, repo, err := originOwnerRepo(root)
	if err != nil {
		return syncOutcome{err: fmt.Errorf("remote: %w", err)}
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
		return syncOutcome{err: fmt.Errorf("read plan: %w", err)}
	}
	beforeBytes, err := toml.Marshal(p)
	if err != nil {
		return syncOutcome{err: fmt.Errorf("marshal before: %w", err)}
	}
	var before plan
	if err := toml.Unmarshal(beforeBytes, &before); err != nil {
		return syncOutcome{err: fmt.Errorf("clone plan: %w", err)}
	}

	prInfo, err := prForBranch(owner, repo, branch)
	if err != nil {
		return syncOutcome{err: fmt.Errorf("find pr: %w", err)}
	}

	if prInfo != nil {
		log.Debug("found pr", log.Args("url", prInfo.URL, "state", prInfo.State))
		prData, closes, err := pr(prInfo.URL)
		if err != nil {
			return syncOutcome{err: fmt.Errorf("fetch pr: %w", err)}
		}
		prData.State = prInfo.State
		// GitHub returns "unknown" for mergeable when it hasn't computed
		// the value yet (lazy). Preserve the previous non-empty value —
		// but only while the PR is still OPEN, because a merged or
		// closed PR intentionally reports "unknown" and we want the
		// transition to be visible.
		prevMergeable := firstPR(p).Mergeable
		if prInfo.State == "OPEN" && prData.Mergeable == "unknown" && prevMergeable != "" && prevMergeable != "unknown" {
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

	// PR reached a terminal state (MERGED or CLOSED) → flag for cleanup.
	// We deliberately don't gate on `p.Status != statusClosed`: the
	// worktree existing on disk is the invariant, and if it's here we
	// want it gone. Removal is effectively idempotent because after
	// cleanup the worktree is off the list and won't be synced again.
	autoClose := false
	closeReason := ""
	if bPR := firstPR(p); bPR.State == "MERGED" || bPR.State == "CLOSED" {
		p.Status = statusClosed
		autoClose = true
		closeReason = strings.ToLower(bPR.State) // "merged" / "closed"
	}

	afterBytes, err := toml.Marshal(p)
	if err != nil {
		return syncOutcome{err: fmt.Errorf("marshal after: %w", err)}
	}
	if bytes.Equal(beforeBytes, afterBytes) {
		return syncOutcome{} // no-op
	}
	if dryRun {
		return syncOutcome{changed: true, note: diffPlans(before, p), autoClose: autoClose, closeReason: closeReason}
	}
	if err := writePlan(p); err != nil {
		return syncOutcome{err: fmt.Errorf("write plan: %w", err)}
	}
	return syncOutcome{changed: true, note: diffPlans(before, p), autoClose: autoClose}
}

// diffPlans produces a short human-readable summary of what changed between
// two plans. Used for the sync summary "note" column.
func diffPlans(a, b plan) string {
	var parts []string

	if a.Status != b.Status {
		parts = append(parts, fmt.Sprintf("status: %s → %s", nonEmpty(string(a.Status)), nonEmpty(string(b.Status))))
	}

	aPR, bPR := firstPR(a), firstPR(b)

	// PR: appearance
	switch {
	case aPR.URL == "" && bPR.URL != "":
		parts = append(parts, "new pr")
	case aPR.URL != "" && bPR.URL == "":
		parts = append(parts, "pr removed")
	}
	// PR: lifecycle transition (OPEN → MERGED / CLOSED). Reported before
	// mergeability so a merge shows up as "pr: OPEN → MERGED" rather than
	// the noisier "pr: clean → unknown".
	if aPR.URL != "" && bPR.URL != "" && aPR.State != bPR.State {
		parts = append(parts, fmt.Sprintf("pr: %s → %s", nonEmpty(aPR.State), nonEmpty(bPR.State)))
	}
	// PR: mergeability (only interesting while OPEN — merged/closed PRs
	// intentionally report "unknown", already covered by the state line).
	if aPR.URL != "" && bPR.URL != "" && bPR.State == "OPEN" && aPR.Mergeable != bPR.Mergeable {
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
