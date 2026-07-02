package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

type newCmd struct {
	Arg string `arg:"positional" help:"'.' for worktree, or 'title' for a task"`

	// initial status for a new task — at most one; default is open
	Open    bool `arg:"-o,--open" help:"new task in open status (default)"`
	Waiting bool `arg:"-w,--waiting" help:"new task in waiting status"`
	Working bool `arg:"-W,--working" help:"new task in working status"`
	Closed  bool `arg:"-c,--closed" help:"new task in closed status"`

	// due date override; empty → tomorrow at midnight
	Due string `arg:"-d,--due" help:"due: 2h, 3d, tomorrow, or YYYY-MM-DD [HH:MM]"`
}

// runNew dispatches on the shape of Arg:
//   - ""    → print usage
//   - "."   → worktree from the current branch
//   - else  → task with that title (status defaults to open, due to
//             tomorrow-at-midnight unless -o/-w/-W/-c or --due override)
func runNew(c *newCmd) error {
	switch c.Arg {
	case "":
		pterm.Info.Println(`usage: work new [-o|-w|-W|-c] [-d <spec>] <arg>

  work new .                       create worktree from the current branch
  work new "title"                 create a task (opens $EDITOR)
  work new -W "title"              create a task in working status
  work new -d 2h "title"           create a task due in 2 hours
  work new -W -d 2h "title"        combine — working, due in 2 hours

Worktrees only come from the current branch — switch branches first, then
run 'work new .'. To navigate an existing worktree, use 'work' or
'work <name>'.`)
		return nil
	case ".":
		return newFromCurrent()
	}
	status, err := pickStatusFlag(c.Open, c.Waiting, c.Working, c.Closed, statusOpen)
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	due, err := parseDue(c.Due)
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	p, err := newTask(c.Arg, status, due)
	if err != nil {
		return err
	}
	pterm.Success.Printfln("task #%s (%s, due %s): %s",
		path.Base(strings.TrimSuffix(p.Path, ".toml")), status,
		p.Due.Format("2006-01-02 15:04"), p.Title)
	return openInEditor(p.Path)
}

// newFromCurrent moves the current branch to ~/w/<repo>/<slug>/ via git worktree.
func newFromCurrent() error {
	branch, err := currentBranch(".")
	if err != nil {
		return fmt.Errorf("new .: %w", err)
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("already on %s, nothing to move", branch)
	}
	repo, err := currentRepoName()
	if err != nil {
		return fmt.Errorf("new .: %w", err)
	}
	dir := path.Join(defaultWorkDir, repo, branchSlug(branch))
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("worktree already exists: %s", dir)
	}
	if err := os.MkdirAll(path.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	// Switch current checkout back to main/master so the branch is free.
	if err := exec.Command("git", "switch", "main").Run(); err != nil {
		if err2 := exec.Command("git", "switch", "master").Run(); err2 != nil {
			return fmt.Errorf("could not switch back to main/master")
		}
	}
	cmd := exec.Command("git", "worktree", "add", dir, branch)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	emitPath(dir)
	return nil
}

// parseDue interprets the -d/--due flag value and returns the resulting
// time.Time. Empty input falls back to config.Task.DefaultDue (parsed
// recursively) and then to "tomorrow" at midnight. Recognized forms:
//
//   - "today" / "tomorrow"              → that day at midnight (local)
//   - "Nd" / "Nw" (int + d or w)        → now + N days/weeks, midnight
//   - go duration ("2h", "1h30m", ...)  → now + duration, precise time
//   - "YYYY-MM-DD"                      → that date at midnight (local)
//   - "YYYY-MM-DD HH:MM"                → that date+time (local)
//
// Fractional day/week values ("1.5d") are not supported — use hours.
var dayWeekRe = regexp.MustCompile(`^(\d+)([dw])$`)

func parseDue(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		if c, err := loadConfig(); err == nil {
			if cfgDue := strings.TrimSpace(c.Task.DefaultDue); cfgDue != "" {
				return parseDueRaw(cfgDue)
			}
		}
		return parseDueRaw("tomorrow")
	}
	return parseDueRaw(raw)
}

// parseDueRaw is the pure parser — no config fallback, no default. Empty
// input is a caller error (parseDue handles empty; direct callers of
// parseDueRaw shouldn't pass "").
func parseDueRaw(raw string) (time.Time, error) {
	now := time.Now()
	midnight := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}
	s := strings.TrimSpace(raw)
	switch strings.ToLower(s) {
	case "":
		return time.Time{}, fmt.Errorf("--due: empty value")
	case "today":
		return midnight(now), nil
	case "tomorrow":
		return midnight(now.AddDate(0, 0, 1)), nil
	}
	if m := dayWeekRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		days := n
		if m[2] == "w" {
			days = n * 7
		}
		return midnight(now.AddDate(0, 0, days)), nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(d), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("--due: unrecognized value %q (want 2h/30m, 3d/1w, today/tomorrow, or YYYY-MM-DD [HH:MM])", raw)
}

// openInEditor launches $EDITOR on file, wiring stdin/stdout/stderr to the
// terminal (via stderr so we don't pollute the stdout cd-path channel).
// Falls back to `vi` when $EDITOR is unset.
func openInEditor(file string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open %s in %s: %w", file, editor, err)
	}
	return nil
}
