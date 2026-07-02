package main

import (
	"fmt"
	"path"
	"sort"
	"time"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
)

// runMerge lets the user tab-select 2+ items (worktrees or tasks) and folds
// them into one primary plan. Rules:
//
//  1. Primary selection (highest wins):
//     a. worktree with a non-empty [pr].url  (branch-with-PR beats everything)
//     b. worktree without a PR
//     c. task
//     Within a tier: newest by plan.mtime wins.
//
//  2. Field merge: tasks[] deduped on string, [[issue]] and [[pr]] unioned
//     on URL. [slack] singleton: primary wins; adopted from src only when
//     primary's slack URL is empty.
//
//  3. Cleanup: task files move to ~/w/t/closed/. Worktrees go through
//     `git worktree remove`; git refuses dirty trees, in which case we
//     print a warning and leave the worktree on disk for the user to
//     resolve (the primary is already written, so no data is lost).
func runMerge(_ *mergeCmd) error {
	spinner, err := pterm.DefaultSpinner.WithText("loading").Start()
	if err != nil {
		return fmt.Errorf("merge: spinner: %w", err)
	}
	items, err := loadInventory(true, true)
	if sErr := spinner.Stop(); sErr != nil {
		return fmt.Errorf("merge: spinner stop: %w", sErr)
	}
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	if len(items) < 2 {
		pterm.Info.Println("merge: need at least 2 items in inventory")
		return nil
	}

	labels := formatLabels(items)
	byLabel := make(map[string]inventoryItem, len(items))
	for i, it := range items {
		byLabel[labels[i]] = it
	}

	sel, err := pterm.DefaultInteractiveMultiselect.
		WithOptions(labels).
		WithFilter(true).
		WithMaxHeight(20).
		WithKeySelect(keys.Tab).
		WithKeyConfirm(keys.Enter).
		Show()
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	if len(sel) < 2 {
		pterm.Info.Println("merge: need at least 2 selections")
		return nil
	}

	picks := make([]inventoryItem, 0, len(sel))
	for _, label := range sel {
		picks = append(picks, byLabel[label])
	}

	// Load every pick's plan once, then pass the cached plans into the tier /
	// merge helpers so each file is read a single time even if consulted for
	// tier, mtime, kind-label, and merge-source.
	cache, err := loadPlansForPicks(picks)
	if err != nil {
		return fmt.Errorf("merge: load plans: %w", err)
	}
	primary, others := pickMergePrimary(picks, cache)
	primaryPlan := cache[primary.key()]

	pterm.Info.Printfln("primary: %s (%s)", itemLabel(primary), itemKindLabel(primary, cache))
	for _, o := range others {
		pterm.Info.Printfln("  merging in: %s", itemLabel(o))
	}
	if !confirm(fmt.Sprintf("merge %d item(s) into primary?", len(others))) {
		return fmt.Errorf("merge: cancelled")
	}

	for _, o := range others {
		mergePlanFields(&primaryPlan, cache[o.key()])
	}
	if err := writePlan(primaryPlan); err != nil {
		return fmt.Errorf("merge: write primary %s: %w", primaryPlan.Path, err)
	}
	pterm.Success.Printfln("primary written: %s", relPath(primaryPlan.Path))

	for _, o := range others {
		if err := closeMergedItem(o); err != nil {
			pterm.Warning.Printfln("cleanup %s: %v", itemLabel(o), err)
			continue
		}
		pterm.Success.Printfln("cleaned up %s", itemLabel(o))
	}
	warnIfBroken()
	return nil
}

// pickMergePrimary chooses the primary per the tier rule and returns
// (primary, others). Ties within a tier resolve to the newest plan.mtime.
// Reads the pre-loaded plan cache; no disk IO here.
func pickMergePrimary(picks []inventoryItem, cache map[string]plan) (inventoryItem, []inventoryItem) {
	type scored struct {
		it    inventoryItem
		tier  int
		mtime time.Time
	}
	scoredPicks := make([]scored, len(picks))
	for i, it := range picks {
		p := cache[it.key()]
		scoredPicks[i] = scored{it: it, tier: itemTier(it, p), mtime: itemMtime(it, p)}
	}
	sort.SliceStable(scoredPicks, func(i, j int) bool {
		a, b := scoredPicks[i], scoredPicks[j]
		if a.tier != b.tier {
			return a.tier > b.tier
		}
		return a.mtime.After(b.mtime)
	})
	primary := scoredPicks[0].it
	others := make([]inventoryItem, 0, len(scoredPicks)-1)
	for _, s := range scoredPicks[1:] {
		others = append(others, s.it)
	}
	return primary, others
}

// itemTier assigns the pick a merge-precedence tier from its already-loaded plan.
func itemTier(it inventoryItem, p plan) int {
	if it.Worktree != nil {
		if firstPR(p).URL != "" {
			return 3
		}
		return 2
	}
	return 1
}

// itemMtime returns the plan file mtime (falls back to worktree mtime).
func itemMtime(it inventoryItem, p plan) time.Time {
	if !p.mtime.IsZero() {
		return p.mtime
	}
	if it.Worktree != nil {
		return it.Worktree.Mtime
	}
	return time.Time{}
}

// loadPlansForPicks reads every pick's plan.toml exactly once. Returns a
// map keyed on item's file path so tier/mtime/kind lookups are cheap.
func loadPlansForPicks(picks []inventoryItem) (map[string]plan, error) {
	out := make(map[string]plan, len(picks))
	for _, it := range picks {
		p, err := readPlan(itemPlanPath(it))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", itemPlanPath(it), err)
		}
		out[it.key()] = p
	}
	return out, nil
}

// itemPlanPath returns the on-disk plan.toml path for either item kind.
func itemPlanPath(it inventoryItem) string {
	if it.Task != nil {
		return it.Task.Path
	}
	if it.Worktree != nil {
		return path.Join(it.Worktree.Path, planFileName)
	}
	return ""
}

func itemKindLabel(it inventoryItem, cache map[string]plan) string {
	if it.Worktree != nil {
		if firstPR(cache[it.key()]).URL != "" {
			return "branch+PR"
		}
		return "branch"
	}
	return "task"
}

// mergePlanFields unions dst.Tasks / dst.Issues with src's; fills in PR/Slack
// on dst only if dst is empty for that block and src has a URL.
func mergePlanFields(dst *plan, src plan) {
	// tasks[] — dedupe by exact string match
	seenTasks := map[string]bool{}
	for _, t := range dst.Tasks {
		seenTasks[t] = true
	}
	for _, t := range src.Tasks {
		if !seenTasks[t] {
			dst.Tasks = append(dst.Tasks, t)
			seenTasks[t] = true
		}
	}
	// [[issue]] — dedupe by url
	seenIssues := map[string]bool{}
	for _, i := range dst.Issues {
		if i.URL != "" {
			seenIssues[i.URL] = true
		}
	}
	for _, i := range src.Issues {
		if i.URL != "" && !seenIssues[i.URL] {
			dst.Issues = append(dst.Issues, i)
			seenIssues[i.URL] = true
		}
	}
	// [[pr]] — union by url. Entries without a URL are dropped as noise
	// (they're the empty stubs from freshly-seeded plans).
	seenPRs := map[string]bool{}
	kept := dst.PRs[:0]
	for _, pr := range dst.PRs {
		if pr.URL == "" || seenPRs[pr.URL] {
			continue
		}
		kept = append(kept, pr)
		seenPRs[pr.URL] = true
	}
	dst.PRs = kept
	for _, pr := range src.PRs {
		if pr.URL == "" || seenPRs[pr.URL] {
			continue
		}
		dst.PRs = append(dst.PRs, pr)
		seenPRs[pr.URL] = true
	}
	// [slack] — kept singular for now. Primary wins; adopt src's iff dst empty.
	if dst.Slack.URL == "" && src.Slack.URL != "" {
		dst.Slack = src.Slack
	}
}

// closeMergedItem retires a non-primary item after its fields are merged.
// Tasks go to ~/w/t/closed/. Worktrees get `git worktree remove` (which
// refuses dirty trees; the picker showed the user what's included, so no
// second confirm).
func closeMergedItem(it inventoryItem) error {
	if it.Task != nil {
		if _, err := moveTask(*it.Task, statusClosed); err != nil {
			return fmt.Errorf("close task: %w", err)
		}
		return nil
	}
	if it.Worktree != nil {
		if err := removeWorktree(*it.Worktree); err != nil {
			return fmt.Errorf("remove worktree: %w", err)
		}
		return nil
	}
	return fmt.Errorf("empty inventory item")
}
