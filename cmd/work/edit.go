package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type editCmd struct {
	Arg string `arg:"positional" help:"'.' for current worktree's plan.toml (default)"`
	All bool   `arg:"-a,--all" help:"batch-edit statuses across filtered items"`

	// type + status filters (only meaningful with --all)
	Tasks     bool `arg:"-t,--task" help:"only edit tasks (with --all)"`
	Worktrees bool `arg:"-b,--branch" help:"only edit worktree branches (with --all)"`
	Open      bool `arg:"-o,--open" help:"status=open filter (with --all)"`
	Waiting   bool `arg:"-w,--waiting" help:"status=waiting filter (with --all)"`
	Working   bool `arg:"-W,--working" help:"status=working filter (with --all)"`
	Closed    bool `arg:"-c,--closed" help:"status=closed filter (with --all)"`
}

func (c *editCmd) statusFilter() map[statusKind]bool {
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
		return nil
	}
	return set
}

const editHeader = `# work edit — change the first char of each line to set that item's status.
#   o = open     w = waiting     W = working     c = closed
#
# save + close to apply. delete a line to skip it. lines starting with # are ignored.
# the path after "#" on each row is the identifier — don't touch it.

`

// runEdit has two modes:
//   - default / "." → open the current worktree's plan.toml in $EDITOR
//   - --all         → open a scratch file listing all filtered items, one
//                     status letter per line; on close, diff + apply per-item
func runEdit(c *editCmd) error {
	if !c.All {
		if c.Arg != "" && c.Arg != "." {
			return fmt.Errorf(`edit: expected "." or --all; got %q`, c.Arg)
		}
		return editCurrentPlan()
	}
	return runEditAll(c)
}

// editCurrentPlan opens the plan.toml of the current worktree in $EDITOR,
// then validates it and reports parse errors immediately.
func editCurrentPlan() error {
	wt, err := currentWorktree()
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}
	planPath := path.Join(wt.Path, planFileName)
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		if !confirmAlways(fmt.Sprintf("no plan.toml in %s. Seed one?", wt)) {
			return fmt.Errorf("edit: no plan.toml")
		}
		if err := seedPlan(planPath, wt.Branch); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}
	if err := openInEditor(planPath); err != nil {
		return err
	}
	if _, err := readPlan(planPath); err != nil {
		pterm.Warning.Printfln("plan.toml has parse errors — run `work validate` for details")
		pterm.Warning.Printfln("  %v", err)
	}
	return nil
}

// runEditAll opens $EDITOR on a filtered inventory rendered as a status-per-row
// scratch file. After the editor exits, we re-read the file, diff against the
// starting state, and apply each changed row.
func runEditAll(c *editCmd) error {
	showWT := !c.Tasks || c.Worktrees
	showCh := !c.Worktrees || c.Tasks

	items, err := loadInventory(showWT, showCh)
	if err != nil {
		return err
	}
	items = filterByStatus(items, c.statusFilter())
	if len(items) == 0 {
		pterm.Info.Println("nothing to edit")
		return nil
	}

	// Snapshot originals keyed by identifier.
	type entry struct {
		item   inventoryItem
		id     string
		origin statusKind
		label  string
	}
	entries := make([]entry, len(items))
	nameW := 0
	for i, it := range items {
		entries[i] = entry{
			item:   it,
			id:     itemID(it),
			origin: itemStatus(it),
			label:  itemLabel(it),
		}
		if l := runeLen(entries[i].label); l > nameW {
			nameW = l
		}
	}

	// Write scratch file.
	tmp, err := os.CreateTemp("", "work-edit-*.txt")
	if err != nil {
		return fmt.Errorf("tmp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(editHeader); err != nil {
		tmp.Close()
		return err
	}
	for _, e := range entries {
		fmt.Fprintf(tmp, "%c  %-*s  # %s\n", statusLetter(e.origin), nameW, e.label, e.id)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("tmp close: %w", err)
	}

	if err := openInEditor(tmpPath); err != nil {
		return fmt.Errorf("editor: %w", err)
	}

	// Parse edited file: map id → new status letter. Missing ids = user deleted
	// that line = leave unchanged.
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("re-read: %w", err)
	}
	updates := make(map[string]byte)
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		body, idPart, ok := strings.Cut(line, "#")
		if !ok {
			pterm.Warning.Printfln("line %d: missing '# <id>' — skipped", lineNo+1)
			continue
		}
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		updates[strings.TrimSpace(idPart)] = body[0]
	}

	// Apply.
	changed := 0
	for _, e := range entries {
		letter, ok := updates[e.id]
		if !ok {
			continue // line was deleted or never matched
		}
		target := statusFromLetter(letter)
		if target == "" {
			pterm.Warning.Printfln("invalid status %q for %s — skipped", string(letter), e.label)
			continue
		}
		if target == e.origin {
			continue
		}
		if err := setStatus(e.item, target); err != nil {
			pterm.Warning.Printfln("FAIL %s: %v", e.label, err)
			continue
		}
		pterm.Success.Printfln("%s: %s → %s", e.label, e.origin, target)
		changed++
	}
	if changed == 0 {
		pterm.Info.Println("no changes")
	}
	warnIfBroken()
	return nil
}

// itemID returns the stable identifier for an item (its filesystem path).
func itemID(it inventoryItem) string {
	switch {
	case it.Worktree != nil:
		return it.Worktree.Path
	case it.Task != nil:
		return it.Task.Path
	}
	return ""
}

// itemStatus returns the current status of an item (worktree reads plan.toml).
func itemStatus(it inventoryItem) statusKind {
	switch {
	case it.Task != nil:
		return it.Task.Status
	case it.Worktree != nil:
		if p, err := readPlan(path.Join(it.Worktree.Path, planFileName)); err == nil {
			return p.Status
		}
		return statusOpen
	}
	return statusOpen
}

// itemLabel returns the user-facing name of an item.
func itemLabel(it inventoryItem) string {
	switch {
	case it.Worktree != nil:
		return it.Worktree.String()
	case it.Task != nil:
		if it.Task.Title != "" {
			return it.Task.Title
		}
		return strings.TrimSuffix(path.Base(it.Task.Path), ".toml")
	}
	return ""
}

// statusLetter returns the single-char code for a status.
func statusLetter(s statusKind) byte {
	switch s {
	case statusOpen:
		return 'o'
	case statusWaiting:
		return 'w'
	case statusWorking:
		return 'W'
	case statusClosed:
		return 'c'
	}
	return 'o'
}

// statusFromLetter reverses statusLetter. Empty return = invalid.
func statusFromLetter(l byte) statusKind {
	switch l {
	case 'o':
		return statusOpen
	case 'w':
		return statusWaiting
	case 'W':
		return statusWorking
	case 'c':
		return statusClosed
	}
	return ""
}
