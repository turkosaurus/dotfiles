package main

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

// filterSpec is the shared set of flags for commands that pick from
// inventory (list, pick, edit, status). Embedded via arg tags so the
// short/long flags appear on each command's --help with one wording.
// Owns the default "closed hidden" rule and the explicit-vs-default
// detection needed by filterInventory's -s union/intersect composition.
type filterSpec struct {
	Tasks     bool `arg:"-t,--task" help:"only offer tasks"`
	Worktrees bool `arg:"-b,--branch" help:"only offer worktree branches"`
	Open      bool `arg:"-o,--open" help:"status=open"`
	Waiting   bool `arg:"-w,--waiting" help:"status=waiting"`
	Working   bool `arg:"-W,--working" help:"status=working"`
	Closed    bool `arg:"-c,--closed" help:"status=closed"`
	All       bool `arg:"-a,--all" help:"include closed items"`
}

// statusFilter returns the effective status set + whether the caller
// set at least one flag explicitly. The bool feeds filterInventory's
// intersect-vs-union rule for -s/--sprint.
//
// Precedence:
//   - --all           → (nil, false)     — everything, no filter
//   - any status flag → (explicit set, true)
//   - no flags        → ({open,waiting,working}, false) — closed hidden
func (f *filterSpec) statusFilter() (map[statusKind]bool, bool) {
	if f.All {
		return nil, false
	}
	set := map[statusKind]bool{}
	if f.Open {
		set[statusOpen] = true
	}
	if f.Waiting {
		set[statusWaiting] = true
	}
	if f.Working {
		set[statusWorking] = true
	}
	if f.Closed {
		set[statusClosed] = true
	}
	if len(set) == 0 {
		return map[statusKind]bool{
			statusOpen:    true,
			statusWaiting: true,
			statusWorking: true,
		}, false
	}
	return set, true
}

// showKinds returns (showWorktrees, showTasks) based on -t/-b. Neither
// flag → both. Used by every embedder to gate loadInventory.
func (f *filterSpec) showKinds() (bool, bool) {
	return !f.Tasks || f.Worktrees, !f.Worktrees || f.Tasks
}

type listCmd struct {
	filterSpec
}

// Nerd-font icons — octicons for the type + status glyphs. All in the
// U+F400–F533 octicon range that your git-branch icon lives in, so if that
// renders, these should too.
const (
	iconWorktree = "" // U+F418 nf-oct-git_branch
	iconTask     = "" // U+F0AE nf-fa-tasks

	iconStatusOpen    = "" // nf-fa-circle_o (empty circle)
	iconStatusWaiting = "" // nf-fa-clock_o — waiting
	iconStatusWorking = "" // nf-oct-issue_opened (small dot in circle) — working
	iconStatusClosed  = "" // nf-fa-check_circle_o (check in circle) — closed
	iconStatusBroken  = "" // nf-fa-times_circle_o (X in circle)
	iconStatusUnknown = "" // nf-fa-question_circle
)

// statusIcon maps a statusKind to a nerd-font glyph. Unknown → · placeholder.
func statusIcon(s statusKind) string {
	switch s {
	case statusOpen:
		return iconStatusOpen
	case statusWaiting:
		return iconStatusWaiting
	case statusWorking:
		return iconStatusWorking
	case statusClosed:
		return iconStatusClosed
	}
	return iconStatusUnknown
}

// inventoryItem is a picker/list entry: exactly one of Worktree, Task is set.
type inventoryItem struct {
	Worktree *worktree
	Task     *plan
}

// key returns a stable identifier for the item — the plan.toml path for
// tasks, or the worktree's path for worktrees. Used to key caches.
func (it inventoryItem) key() string {
	if it.Task != nil {
		return it.Task.Path
	}
	if it.Worktree != nil {
		return it.Worktree.Path
	}
	return ""
}

// row renders an item as columns for the summary table.
func (it inventoryItem) row() []string {
	if it.Worktree != nil {
		return worktreeRow(*it.Worktree)
	}
	return taskRow(*it.Task)
}

// label — fallback if someone bypasses formatLabels.
func (it inventoryItem) label() string {
	r := it.row()
	return fmt.Sprintf("%s  %s  %s  %s", r[0], r[1], r[2], r[3])
}

// formatLabels renders picker labels with the name column padded to the widest
// value. Layout: [type] [name] [status] [age] — name is the filterable column.
// Long name columns are truncated with an ellipsis so labels fit within the
// terminal width (no wrapping into an ugly second line).
func formatLabels(items []inventoryItem) []string {
	if len(items) == 0 {
		return nil
	}
	rows := make([][]string, len(items))
	nameW := 0
	for i, it := range items {
		rows[i] = it.row()
		if l := runeLen(rows[i][1]); l > nameW {
			nameW = l
		}
	}
	// Reserve the fixed columns: icon (1) + 2sp + name + 2sp + status (1) +
	// 2sp + due (up to 4, e.g. "+1mo") — plus a right-edge safety margin.
	// Multiselect prefixes each line with `[ ]` and a space (4 chars) so
	// account for that too.
	term := pterm.GetTerminalWidth()
	const fixed = 1 + 2 + 2 + 1 + 2 + 4 + 4 + 3
	maxName := term - fixed
	if maxName < 10 { // paranoid floor for very narrow terminals
		maxName = 10
	}
	if nameW > maxName {
		nameW = maxName
	}
	labels := make([]string, len(items))
	for i, r := range rows {
		name := truncateRunes(r[1], nameW)
		labels[i] = fmt.Sprintf("%s  %-*s  %s  %s",
			r[0], nameW, name, r[2], r[3])
	}
	return labels
}

// truncateRunes returns s truncated to at most max visible runes,
// replacing the last rune with an ellipsis when clipped. Assumes single-
// width glyphs (matches runeLen).
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

// runeLen counts visible runes (approx — assumes single-width glyphs).
func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// filterByStatus keeps items whose plan.status is in the set. Worktrees
// without a plan.toml (or with an unparseable one) are treated as status
// "open". Run `work validate` to see broken plans explicitly.
// A nil set means no filter (return everything).
func filterByStatus(items []inventoryItem, set map[statusKind]bool) []inventoryItem {
	if set == nil {
		return items
	}
	out := items[:0]
	for _, it := range items {
		if set[itemStatus(it)] {
			out = append(out, it)
		}
	}
	return out
}

// loadInventory returns worktrees and/or tasks per the flags. Callers
// apply status/sprint filters via filterInventory (or applySprintFilter
// directly for callers without status filters) so composition rules
// stay uniform. See filterInventory for the -s union/intersect logic.
//
// Items are sorted by due-date ascending (soonest/overdue first), then
// by mtime descending as a tiebreak. Items with no due date sink to the
// end. See itemDue for how due-date is resolved per item.
func loadInventory(showWT, showCh bool) ([]inventoryItem, error) {
	var items []inventoryItem
	if showWT {
		wts, err := listWorktrees()
		if err != nil {
			return nil, fmt.Errorf("worktrees: %w", err)
		}
		for i := range wts {
			wt := wts[i] // copy so &wt is stable
			items = append(items, inventoryItem{Worktree: &wt})
		}
	}
	if showCh {
		tasks, err := listTasksAll()
		if err != nil {
			return nil, fmt.Errorf("tasks: %w", err)
		}
		for i := range tasks {
			ch := tasks[i]
			items = append(items, inventoryItem{Task: &ch})
		}
	}
	sortByDue(items)
	return items, nil
}

// sortByDue orders items in place: earliest due first, no-due last.
// Tiebreak within same due date (and among no-due items) is mtime
// descending so recently-touched items surface first. Pairs item+due+mt
// into a single struct before sorting so due/mt travel with the item
// during swaps (parallel arrays go stale — sort permutes items but not
// the sidecar slices).
func sortByDue(items []inventoryItem) {
	type keyed struct {
		it  inventoryItem
		due time.Time
		mt  time.Time
	}
	rows := make([]keyed, len(items))
	for i, it := range items {
		d, m := itemDueAndMtime(it)
		rows[i] = keyed{it: it, due: d, mt: m}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		iZero, jZero := rows[i].due.IsZero(), rows[j].due.IsZero()
		if iZero != jZero {
			return !iZero
		}
		if !iZero && !rows[i].due.Equal(rows[j].due) {
			return rows[i].due.Before(rows[j].due)
		}
		return rows[i].mt.After(rows[j].mt)
	})
	for i, r := range rows {
		items[i] = r.it
	}
}

// itemDueAndMtime returns (due, mtime) for the item. For a task both
// come from the plan struct. For a worktree, due is read from
// plan.toml (zero if missing/broken) and mtime is the worktree mtime.
// Batched so sortByDue avoids two plan.toml reads per worktree.
func itemDueAndMtime(it inventoryItem) (time.Time, time.Time) {
	if it.Task != nil {
		return it.Task.Due, it.Task.mtime
	}
	if it.Worktree != nil {
		if p, err := readPlan(path.Join(it.Worktree.Path, planFileName)); err == nil {
			return p.Due, it.Worktree.Mtime
		}
		return time.Time{}, it.Worktree.Mtime
	}
	return time.Time{}, time.Time{}
}

// runList renders a unified table of worktrees + tasks.
// Type flags (--tasks/--worktrees) narrow by kind; status flags
// (-o/-w/-W/-c) narrow by status.
func runList(c *listCmd) error {
	showWT := !c.Tasks || c.Worktrees
	showCh := !c.Worktrees || c.Tasks

	spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
	items, err := loadInventory(showWT, showCh)
	_ = spinner.Stop()
	if err != nil {
		return err
	}
	set, explicit := c.statusFilter()
	items = filterInventory(items, set, explicit)
	if len(items) == 0 {
		pterm.Info.Println("nothing found")
		return nil
	}

	rows := pterm.TableData{{"", "name", "", "due"}}
	for _, it := range items {
		rows = append(rows, it.row())
	}
	truncateTableNames(rows)
	return pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
}

// truncateTableNames clips the name column (index 1) of each row so the
// table fits the current terminal width. Fixed overhead is icons + " | "
// separators + due (up to 4 chars, e.g. "+1mo") + a right-edge margin.
// The header row is included in the width scan so the "name" header
// itself doesn't get squeezed.
func truncateTableNames(rows pterm.TableData) {
	term := pterm.GetTerminalWidth()
	const fixed = 1 + 3 + 3 + 1 + 3 + 4 + 1
	maxName := term - fixed
	if maxName < 10 {
		maxName = 10
	}
	for i := range rows {
		if len(rows[i]) < 2 {
			continue
		}
		if runeLen(rows[i][1]) > maxName {
			rows[i][1] = truncateRunes(rows[i][1], maxName)
		}
	}
}

// row schema: [type_icon, name, status_icon, age]
//   - type_icon: git-branch (worktree) or tasks (task)
//   - name: repo:branch (worktree) or title (task) — the filterable column
//   - status_icon: nerd-font glyph for open/pending/done (· if unknown)
//   - age: relative time — file mtime for both worktrees and tasks

func worktreeRow(wt worktree) []string {
	status := iconStatusUnknown
	name := wt.String()
	var due time.Time
	pp := path.Join(wt.Path, planFileName)
	if _, err := os.Stat(pp); err == nil {
		if p, err := readPlan(pp); err == nil {
			status = statusIcon(p.Status)
			due = p.Due
			// Append the first linked issue's title so the picker's
			// filter matches on it (branch slug alone won't hit
			// searches for the underlying issue name).
			if title := firstIssueTitle(p); title != "" {
				name = fmt.Sprintf("%s · %s", name, title)
			}
		} else {
			status = iconStatusBroken
		}
	}
	return []string{iconWorktree, name, status, dueOffset(due)}
}

// firstIssueTitle returns the first non-empty [[issue]].title from p, or
// "" if none. Used to make picker labels searchable by issue name in
// addition to branch/repo slug.
func firstIssueTitle(p plan) string {
	for _, i := range p.Issues {
		if i.Title != "" {
			return i.Title
		}
	}
	return ""
}

func taskRow(ch plan) []string {
	name := strings.TrimSuffix(path.Base(ch.Path), ".toml")
	title := ch.Title
	if title == "" {
		title = name
	}
	status := statusIcon(ch.Status)
	if ch.broken {
		status = iconStatusBroken
		title = name + " (broken)"
	}
	return []string{iconTask, title, status, dueOffset(ch.Due)}
}

// listTasksAll walks open/waiting/working/closed and returns every task
// plan. Runs reconcileTaskLocations first so any file whose status field
// disagrees with its parent directory (hand-edit via `work edit` or yq)
// is relocated before the walk — otherwise a moved file could appear
// twice in the result.
func listTasksAll() ([]plan, error) {
	reconcileTaskLocations()
	var all []plan
	for _, s := range []statusKind{statusOpen, statusWaiting, statusWorking, statusClosed} {
		tasks, err := listTasks(s)
		if err != nil {
			return nil, err
		}
		all = append(all, tasks...)
	}
	return all, nil
}
