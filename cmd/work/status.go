package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type statusCmd struct{}

// runStatus prints the current directory's plan.toml. When there is no
// plan.toml in cwd, falls through to `work ls -wW` — a filtered list of
// waiting + working items.
func runStatus(_ *statusCmd) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	planPath := path.Join(cwd, planFileName)
	if _, err := os.Stat(planPath); err != nil {
		return runList(&listCmd{filterSpec: filterSpec{
			Waiting: true,
			Working: true,
		}})
	}
	p, err := readPlan(planPath)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	return printPlanStatus(p)
}

// statusStyle maps a plan status to its display color.
//
//	open    → white
//	working → green
//	waiting → yellow
//	closed  → red
func statusStyle(s statusKind) *pterm.Style {
	switch s {
	case statusWorking:
		return pterm.NewStyle(pterm.FgGreen)
	case statusWaiting:
		return pterm.NewStyle(pterm.FgYellow)
	case statusClosed:
		return pterm.NewStyle(pterm.FgRed)
	}
	return pterm.NewStyle(pterm.FgWhite)
}

// printPlanStatus renders p to stdout: a status-colored title line,
// a bullet list of tasks (always open-colored — tasks in tasks[] are
// open by definition), and issue/PR sections with per-item colors and
// URLs on their own uncolored lines so they can be copied.
func printPlanStatus(p plan) error {
	st := statusStyle(p.Status)
	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	st.Printfln("%s %s", statusIcon(p.Status), title)

	if len(p.Tasks) > 0 {
		openStyle := statusStyle(statusOpen)
		items := make([]pterm.BulletListItem, 0, len(p.Tasks))
		for _, t := range p.Tasks {
			line := strings.SplitN(strings.TrimSpace(t), "\n", 2)[0]
			if line == "" {
				continue
			}
			items = append(items, pterm.BulletListItem{
				Level:       0,
				Text:        line,
				TextStyle:   openStyle,
				BulletStyle: openStyle,
			})
		}
		if len(items) > 0 {
			if err := pterm.DefaultBulletList.WithItems(items).Render(); err != nil {
				return err
			}
		}
	}

	if len(p.Issues) > 0 {
		pterm.Println()
		for _, iss := range p.Issues {
			label := issueStatusLabel(iss)
			statusStyle(issueStatusKind(iss)).Printfln("%s (%s)", iss.Title, label)
			pterm.Println(iss.URL)
		}
	}

	if len(p.PRs) > 0 {
		pterm.Println()
		for _, pr := range p.PRs {
			label := pr.Mergeable
			switch strings.ToUpper(pr.State) {
			case "MERGED", "CLOSED":
				label = strings.ToLower(pr.State)
			}
			if label == "" {
				label = pr.State
			}
			statusStyle(prStatusKind(pr)).Printfln("%s (%s)", pr.Title, label)
			pterm.Println(pr.URL)
		}
	}
	return nil
}

// issueStatusLabel prefers the project-board column when set, since
// that's the sprint status (e.g. "In Progress"). Falls back to a plain
// open/closed derived from the boolean.
func issueStatusLabel(iss Issue) string {
	if iss.Project.Status != "" {
		return iss.Project.Status
	}
	if iss.Closed {
		return "closed"
	}
	return "open"
}

// issueStatusKind maps an issue's project column (or closed flag) to
// one of our four statusKinds for coloring. Project columns are
// free-form on GitHub, so we match on common labels case-insensitively.
func issueStatusKind(iss Issue) statusKind {
	if iss.Closed {
		return statusClosed
	}
	switch strings.ToLower(iss.Project.Status) {
	case "in progress", "in review", "working":
		return statusWorking
	case "waiting", "blocked", "on hold":
		return statusWaiting
	case "done", "closed":
		return statusClosed
	}
	return statusOpen
}

// prStatusKind maps a PR's mergeable/state to one of our four
// statusKinds for coloring. mergeable wins when set; state is the
// fallback (MERGED/CLOSED → closed, else open).
func prStatusKind(pr PR) statusKind {
	switch strings.ToLower(pr.Mergeable) {
	case "clean", "mergeable":
		return statusWorking
	case "conflicting", "dirty":
		return statusWaiting
	case "draft":
		return statusOpen
	}
	switch strings.ToUpper(pr.State) {
	case "MERGED", "CLOSED":
		return statusClosed
	}
	return statusOpen
}
