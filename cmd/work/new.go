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

	// -r/-b: create a fresh worktree by branching off an updated main.
	// -b requires a branch name; -r defaults to the current repo.
	Repo   string `arg:"-r,--repo" help:"repo name under <path.repos> (defaults to current)"`
	Branch string `arg:"-b,--branch" help:"create new branch off current main and set up its worktree"`

	// initial status for a new task — at most one; default is waiting
	// (new tasks are waiting on prioritization until promoted)
	Open    bool `arg:"-o,--open" help:"new task in open status"`
	Waiting bool `arg:"-w,--waiting" help:"new task in waiting status (default)"`
	Working bool `arg:"-W,--working" help:"new task in working status"`
	Closed  bool `arg:"-c,--closed" help:"new task in closed status"`

	// due date override; empty → no due date set
	Due string `arg:"-d,--due" help:"due: 2h, 3d, tomorrow, or YYYY-MM-DD [HH:MM]"`
}

// runNew dispatches on the shape of Arg / flags:
//   - -b <branch>          → branch off updated main and set up worktree
//   - Arg == ""            → print usage
//   - Arg == "."           → worktree from the current branch
//   - else                 → task with that title (status defaults to
//     waiting — waiting on prioritization; due is unset unless --due
//     is passed)
func runNew(c *newCmd) error {
	if c.Branch != "" {
		return newFromBranch(c.Repo, c.Branch)
	}
	switch c.Arg {
	case "":
		pterm.Info.Println(`usage: work new [-o|-w|-W|-c] [-d <spec>] <arg>
       work new [-r <repo>] -b <branch>

  work new .                       create worktree from the current branch
  work new -b feat/x               branch off main in current repo, worktree it
  work new -r foo -b feat/x        same, but in <path.repos>/foo
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
	status, err := pickStatusFlag(c.Open, c.Waiting, c.Working, c.Closed, statusWaiting)
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
	num := path.Base(strings.TrimSuffix(p.Path, ".toml"))
	if p.Due.IsZero() {
		pterm.Success.Printfln("task #%s (%s): %s", num, status, p.Title)
	} else {
		pterm.Success.Printfln("task #%s (%s, due %s): %s",
			num, status, p.Due.Format("2006-01-02 15:04"), p.Title)
	}
	return openInEditor(p.Path)
}

// newFromBranch creates a fresh worktree by branching off origin's default
// branch (main, else master) in the repo clone. repo defaults to the current
// repo when empty; the clone is looked up under <path.repos>/<repo>. Fetches
// origin first so the new branch is based on current upstream.
func newFromBranch(repo, branch string) error {
	if branch == "" {
		return fmt.Errorf("new: -b <branch> required")
	}
	if repo == "" {
		r, err := currentRepoName()
		if err != nil {
			return fmt.Errorf("new -b: no -r and not inside a repo: %w", err)
		}
		repo = r
	}

	cfg, _ := loadConfig()
	reposDir := cfg.Path.Repos
	if reposDir == "" {
		reposDir = path.Join(os.Getenv("HOME"), "p")
	}
	repoDir := path.Join(reposDir, repo)
	if fi, err := os.Stat(repoDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("new -b: repo %q not found under %s", repo, reposDir)
	}

	wtDir := path.Join(defaultWorkDir, repo, branchSlug(branch))
	if _, err := os.Stat(wtDir); err == nil {
		return fmt.Errorf("worktree already exists: %s", wtDir)
	}
	if err := os.MkdirAll(path.Dir(wtDir), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	fetch := exec.Command("git", "-C", repoDir, "fetch", "origin")
	fetch.Stdout = os.Stderr
	fetch.Stderr = os.Stderr
	if err := fetch.Run(); err != nil {
		return fmt.Errorf("git fetch origin (in %s): %w", repoDir, err)
	}

	base := "origin/main"
	if err := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", base).Run(); err != nil {
		base = "origin/master"
		if err := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", base).Run(); err != nil {
			return fmt.Errorf("neither origin/main nor origin/master exists in %s", repoDir)
		}
	}

	add := exec.Command("git", "-C", repoDir,
		"worktree", "add", "--no-track", "-b", branch, wtDir, base)
	add.Stdout = os.Stderr
	add.Stderr = os.Stderr
	if err := add.Run(); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}

	planPath := path.Join(wtDir, planFileName)
	if err := seedPlan(planPath, branch); err != nil {
		pterm.Warning.Printfln("seed %s: %v", planPath, err)
	}

	pterm.Success.Printfln("branched %s off %s → %s", branch, base, wtDir)
	emitPath(wtDir)
	return nil
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
// time.Time. Empty input returns the zero Time (no default is applied —
// callers that want a fallback must supply it themselves). Recognized
// forms:
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
		return time.Time{}, nil
	}
	return parseDueRaw(raw)
}

// parseDueRaw is the pure parser — no empty-string handling. Callers that
// need "empty means no due" should go through parseDue, which returns the
// zero Time for empty input.
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
