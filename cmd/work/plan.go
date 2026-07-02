package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// legacyPRHeader matches `[pr]` on its own line — the old single-table shape.
// upgradeLegacyPR rewrites it to `[[pr]]` so the field can unmarshal as []PR.
// Guard: if the file already contains `[[pr]]`, migration is skipped; this
// avoids rewriting `[pr]` occurrences inside multi-line string literals like
// `tasks[]` blocks.
var legacyPRHeader = regexp.MustCompile(`(?m)^\[pr\]$`)

func upgradeLegacyPR(data []byte) []byte {
	if bytes.Contains(data, []byte("[[pr]]")) {
		return data
	}
	return legacyPRHeader.ReplaceAll(data, []byte("[[pr]]"))
}

// firstPR returns the first PR in a plan, or a zero PR if none.
// Convenience for sites that used to read the singular p.PR field.
func firstPR(p plan) PR {
	if len(p.PRs) == 0 {
		return PR{}
	}
	return p.PRs[0]
}

// upsertPR replaces the entry with matching URL in prs, or appends when no
// match. Used by sync to keep the branch's PR fresh without adding duplicates.
func upsertPR(prs []PR, in PR) []PR {
	for i, existing := range prs {
		if existing.URL == in.URL {
			prs[i] = in
			return prs
		}
	}
	return append(prs, in)
}

// compactPlan drops entries from p's array fields that carry no meaningful
// data (blank URLs are the tell — they're stubs from seeded plans). Returns
// a copy — the caller's plan is not mutated. Applied at write time to keep
// plan.toml files free of `[[issue]] url = ''` clutter.
func compactPlan(p plan) plan {
	issues := p.Issues[:0:0]
	for _, i := range p.Issues {
		if i.URL != "" {
			issues = append(issues, i)
		}
	}
	prs := p.PRs[:0:0]
	for _, pr := range p.PRs {
		if pr.URL != "" {
			prs = append(prs, pr)
		}
	}
	p.Issues = issues
	p.PRs = prs
	return p
}

type statusKind string

const (
	statusOpen    statusKind = "open"
	statusWaiting statusKind = "waiting"
	statusWorking statusKind = "working"
	statusClosed  statusKind = "closed"

	planFileName             = "plan.toml"
	planFileMode os.FileMode = 0o644
)

type plan struct {
	Title  string     `toml:"title"`
	Status statusKind `toml:"status"`
	Due    time.Time  `toml:"due"`
	Tasks  []string   `toml:"tasks"`
	Slack  slack      `toml:"slack"`
	Issues []Issue    `toml:"issue"`
	PRs    []PR       `toml:"pr"`
	Path   string     `toml:"path"` // path to plan

	broken bool      `toml:"-"` // in-memory only: true if this plan couldn't be parsed
	mtime  time.Time `toml:"-"` // in-memory only: file mtime, used for age display
}

type slack struct {
	Title    string `toml:"title"`
	URL      string `toml:"url"`
	Body     string `toml:"body"`
	Waiting  bool   `toml:"waiting"`
	Resolved bool   `toml:"resolved"`
}

type Issue struct {
	Title  string `toml:"title"`
	URL    string `toml:"url"`
	Closed bool   `toml:"closed"`
	// Project records which board this issue was pulled from and its column
	// at the time of sync. Populated by sprint sync so future runs can
	// detect column moves without re-fetching the full board.
	Project IssueProject `toml:"project"`
}

// IssueProject is the source board + column for an issue, when it came from
// a sprint sync. Omitted from output if URL is empty (compactPlan drops
// zero-value issues; a bare project with no url stays inside a valid issue).
type IssueProject struct {
	URL    string `toml:"url"`
	Status string `toml:"status"`
}

type PR struct {
	Title     string    `toml:"title"`
	Mergeable string    `toml:"mergeable"`
	URL       string    `toml:"url"`
	// State is the PR's lifecycle: OPEN / CLOSED / MERGED. Populated by
	// syncRoot from prForBranch. diffPlans watches transitions so merged
	// PRs surface as "pr: OPEN → MERGED" in the sync line.
	State    string    `toml:"state"`
	Comments []comment `toml:"comment"`
}

type comment struct {
	Title   string     `toml:"title"`
	Status  statusKind `toml:"status"`
	Source  string     `toml:"source"`
	Author  string     `toml:"author"`
	Thread  string     `toml:"thread"`
	FixRef  string     `toml:"fix_ref"`
	Comment string     `toml:"comment"`
	Plan    string     `toml:"plan"`
	Reply   string     `toml:"reply"`
}

func defaultPlan(title string) plan {
	due := time.Now().AddDate(0, 0, defaultDaysDue)
	return plan{
		Title:  title,
		Status: statusOpen,
		Due:    time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.Local),
	}
}

func readPlan(planPath string) (plan, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return plan{}, fmt.Errorf("read plan %q: %w", planPath, err)
	}
	data = upgradeLegacyPR(data)
	var p plan
	if err := toml.Unmarshal(data, &p); err != nil {
		return plan{}, fmt.Errorf("parse plan %q: %w", planPath, err)
	}
	// The file's own actual location wins over any stored `path` field —
	// files can be moved, and the on-disk location is ground truth.
	p.Path = planPath
	if fi, err := os.Stat(planPath); err == nil {
		p.mtime = fi.ModTime()
	}
	return p, nil
}

// relToWork returns p relative to defaultWorkDir when p is under it, else p
// unchanged. Used at write-time to keep plan files portable — an absolute
// `/Users/scrubjay/w/t/1.toml` becomes `t/1.toml`. There is no inverse
// helper: readPlan overrides `p.Path` from the file's on-disk location, so
// callers never see the stored relative form in memory.
func relToWork(p string) string {
	if defaultWorkDir == "" {
		return p
	}
	prefix := defaultWorkDir + "/"
	if p == defaultWorkDir {
		return "."
	}
	if len(p) > len(prefix) && p[:len(prefix)] == prefix {
		return p[len(prefix):]
	}
	return p
}

func writePlan(p plan) error {
	if p.Path == "" {
		return fmt.Errorf("write plan: empty path")
	}
	absPath := p.Path
	p = compactPlan(p)
	// Store the plan's own path relative to ~/w so files are portable across
	// machines. Write to the absolute location, but marshal with the
	// relative form. In-memory callers keep the absolute p.Path — readPlan
	// resets it from the file's on-disk location.
	p.Path = relToWork(absPath)
	data, err := toml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(absPath, data, planFileMode); err != nil {
		return fmt.Errorf("write plan %q: %w", absPath, err)
	}
	return nil
}

