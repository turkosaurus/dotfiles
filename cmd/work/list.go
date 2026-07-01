package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type listCmd struct {
	Chores    bool `arg:"-c,--chores" help:"only show chores"`
	Worktrees bool `arg:"-w,--worktrees" help:"only show worktrees"`
}

// Nerd-font icons — octicons for the type + status glyphs. All in the
// U+F400–F533 octicon range that your git-branch icon lives in, so if that
// renders, these should too.
const (
	iconWorktree = "" // U+F418 nf-oct-git_branch
	iconChore    = "" // U+F0AE nf-fa-tasks

	iconStatusOpen    = "" // nf-fa-circle_o (empty circle)
	iconStatusPending = "" // nf-oct-issue_opened (small dot in circle)
	iconStatusDone    = "" // nf-fa-check_circle_o (check in circle)
	iconStatusBroken  = "" // nf-fa-times_circle_o (X in circle)
	iconStatusUnknown = "" // nf-fa-question_circle
)

// statusIcon maps a statusKind to a nerd-font glyph. Empty status → · placeholder.
func statusIcon(s statusKind) string {
	switch s {
	case statusOpen:
		return iconStatusOpen
	case statusPending:
		return iconStatusPending
	case statusDone:
		return iconStatusDone
	}
	return iconStatusUnknown
}

// inventoryItem is a picker/list entry: exactly one of Worktree, Chore is set.
type inventoryItem struct {
	Worktree *worktree
	Chore    *plan
}

// row renders an item as columns for the summary table.
func (it inventoryItem) row() []string {
	if it.Worktree != nil {
		return worktreeRow(*it.Worktree)
	}
	return choreRow(*it.Chore)
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

// loadInventory returns worktrees and/or chores per the flags.
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
		chores, err := listChoresAll()
		if err != nil {
			return nil, fmt.Errorf("chores: %w", err)
		}
		for i := range chores {
			ch := chores[i]
			items = append(items, inventoryItem{Chore: &ch})
		}
	}
	return items, nil
}

// runList renders a unified table of worktrees + chores. -c/-w narrow the view.
func runList(c *listCmd) error {
	showWT := !c.Chores || c.Worktrees
	showCh := !c.Worktrees || c.Chores

	spinner, _ := pterm.DefaultSpinner.WithText("loading").Start()
	items, err := loadInventory(showWT, showCh)
	_ = spinner.Stop()
	if err != nil {
		return err
	}
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
//   - type_icon: git-branch (worktree) or tasks (chore)
//   - name: repo:branch (worktree) or title (chore) — the filterable column
//   - status_icon: nerd-font glyph for open/pending/done (· if unknown)
//   - age: relative time (mtime for worktrees, due for chores)

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

func choreRow(ch plan) []string {
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
	return []string{iconChore, title, status, timeAgo(localDateAsTime(ch.Due))}
}

// listChoresAll walks open/pending/done and returns every chore plan.
func listChoresAll() ([]plan, error) {
	var all []plan
	for _, s := range []statusKind{statusOpen, statusPending, statusDone} {
		chores, err := listChores(s)
		if err != nil {
			return nil, err
		}
		all = append(all, chores...)
	}
	return all, nil
}
