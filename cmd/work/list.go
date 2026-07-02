package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type listCmd struct {
	// type filters
	Tasks     bool `arg:"-t,--task" help:"show only tasks"`
	Worktrees bool `arg:"-b,--branch" help:"show only worktree branches"`

	// status filters — combinable. No flags = default (open+waiting+working;
	// closed is hidden). --all overrides and shows every status including closed.
	Open    bool `arg:"-o,--open" help:"status=open"`
	Waiting bool `arg:"-w,--waiting" help:"status=waiting"`
	Working bool `arg:"-W,--working" help:"status=working"`
	Closed  bool `arg:"-c,--closed" help:"status=closed"`
	All     bool `arg:"-a,--all" help:"show every status, including closed"`
}

// statusFilter returns the set of statuses to include based on the flags, or
// nil if no filter is active (include everything). Precedence:
//   - --all           → nil (everything)
//   - any status flag → the explicit union
//   - no flags        → open + waiting + working (closed hidden)
func (c *listCmd) statusFilter() map[statusKind]bool {
	if c.All {
		return nil
	}
	set := map[statusKind]bool{}
	if c.Open {
		set[statusOpen] = true
	}
	if c.Waiting {
		set[statusWaiting] = true
	}
	if c.Working {
		set[statusWorking] = true
	}
	if c.Closed {
		set[statusClosed] = true
	}
	if len(set) == 0 {
		return map[statusKind]bool{
			statusOpen:    true,
			statusWaiting: true,
			statusWorking: true,
		}
	}
	return set
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
	labels := make([]string, len(items))
	for i, r := range rows {
		labels[i] = fmt.Sprintf("%s  %-*s  %s  %s",
			r[0], nameW, r[1], r[2], r[3])
	}
	return labels
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

// loadInventory returns worktrees and/or tasks per the flags. The global
// -p/--project filter (via applyProjectFilter) is applied at the end so
// every caller — list, rm, promote, merge, status, edit — honors it.
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
	return applySprintFilter(applyProjectFilter(items)), nil
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
	items = filterByStatus(items, c.statusFilter())
	if len(items) == 0 {
		pterm.Info.Println("nothing found")
		return nil
	}

	rows := pterm.TableData{{"", "name", "", "age"}}
	for _, it := range items {
		rows = append(rows, it.row())
	}
	return pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
}

// row schema: [type_icon, name, status_icon, age]
//   - type_icon: git-branch (worktree) or tasks (task)
//   - name: repo:branch (worktree) or title (task) — the filterable column
//   - status_icon: nerd-font glyph for open/pending/done (· if unknown)
//   - age: relative time — file mtime for both worktrees and tasks

func worktreeRow(wt worktree) []string {
	status := iconStatusUnknown
	pp := path.Join(wt.Path, planFileName)
	if _, err := os.Stat(pp); err == nil {
		if p, err := readPlan(pp); err == nil {
			status = statusIcon(p.Status)
		} else {
			status = iconStatusBroken
		}
	}
	return []string{iconWorktree, wt.String(), status, timeAgo(wt.Mtime)}
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
	return []string{iconTask, title, status, timeAgo(ch.mtime)}
}

// listTasksAll walks open/waiting/working/closed and returns every task plan.
func listTasksAll() ([]plan, error) {
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
