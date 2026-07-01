package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pterm/pterm"
)

const choreCounterFile = ".counter"

// setupDirs ensures defaultWorkDir, defaultChoreDir, and the open/pending/done
// chore subdirs exist, and initializes .counter to 1 if missing. If anything
// needs creating and assumeYes is false, prompts for confirmation first.
func setupDirs() error {
	dirs := []string{
		defaultWorkDir,
		defaultChoreDir,
		choreDir(statusOpen),
		choreDir(statusPending),
		choreDir(statusDone),
	}
	var missing []string
	for _, d := range dirs {
		_, err := os.Stat(d)
		switch {
		case err == nil:
			log.Debug("directory exists", log.Args("path", d))
		case os.IsNotExist(err):
			missing = append(missing, d)
		default:
			return fmt.Errorf("stat %s: %w", d, err)
		}
	}

	if len(missing) > 0 {
		for _, d := range missing {
			pterm.Warning.Printfln("missing: %s", d)
		}
		if !confirm("create these directories?") {
			return fmt.Errorf("setup cancelled")
		}
		for _, d := range missing {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", d, err)
			}
			pterm.Success.Printfln("created %s", d)
		}
	}

	cpath := path.Join(defaultChoreDir, choreCounterFile)
	_, err := os.Stat(cpath)
	switch {
	case err == nil:
		log.Debug("counter exists", log.Args("path", cpath))
	case os.IsNotExist(err):
		if err := os.WriteFile(cpath, []byte("1\n"), planFileMode); err != nil {
			return fmt.Errorf("init counter %s: %w", cpath, err)
		}
		pterm.Success.Printfln("initialized %s", cpath)
	default:
		return fmt.Errorf("stat counter: %w", err)
	}
	return nil
}

// choreDir returns the directory for chores with the given status.
func choreDir(s statusKind) string {
	return path.Join(defaultChoreDir, string(s))
}

// nextChoreNum atomically reads-and-increments the chore counter under an
// exclusive file lock so concurrent invocations never hand out the same N.
// Missing counter file starts the sequence at 1.
func nextChoreNum() (int, error) {
	if err := os.MkdirAll(defaultChoreDir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir chore root: %w", err)
	}
	cpath := path.Join(defaultChoreDir, choreCounterFile)

	f, err := os.OpenFile(cpath, os.O_RDWR|os.O_CREATE, planFileMode)
	if err != nil {
		return 0, fmt.Errorf("open counter %s: %w", cpath, err)
	}
	defer f.Close()

	// Advisory exclusive lock; blocks until acquired.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("lock counter %s: %w", cpath, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	data, err := io.ReadAll(f)
	if err != nil {
		return 0, fmt.Errorf("read counter %s: %w", cpath, err)
	}
	n := 1
	if s := strings.TrimSpace(string(data)); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("parse counter %q: %w", s, err)
		}
		n = v
	}

	if _, err := f.Seek(0, 0); err != nil {
		return 0, fmt.Errorf("seek counter: %w", err)
	}
	if err := f.Truncate(0); err != nil {
		return 0, fmt.Errorf("truncate counter: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%d\n", n+1); err != nil {
		return 0, fmt.Errorf("write counter: %w", err)
	}
	return n, nil
}

// newChore allocates the next chore number and writes a default plan file
// to the `open` subdirectory. Returns the populated plan.
func newChore(title string) (plan, error) {
	n, err := nextChoreNum()
	if err != nil {
		return plan{}, fmt.Errorf("new chore: %w", err)
	}
	dir := choreDir(statusOpen)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return plan{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	p := defaultPlan(title)
	p.Path = path.Join(dir, fmt.Sprintf("%d.toml", n))
	if err := writePlan(p); err != nil {
		return plan{}, fmt.Errorf("write chore %d: %w", n, err)
	}
	return p, nil
}

// listChores returns all chore plans in the given status directory, sorted by
// filename (so by chore number). Files that fail to parse are returned as
// placeholder plans with broken=true so the caller can still show them.
func listChores(s statusKind) ([]plan, error) {
	matches, err := filepath.Glob(path.Join(choreDir(s), "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("glob chores: %w", err)
	}
	var chores []plan
	for _, m := range matches {
		p, err := readPlan(m)
		if err != nil {
			// Placeholder so the caller can still surface the broken file.
			chores = append(chores, plan{Path: m, broken: true})
			continue
		}
		chores = append(chores, p)
	}
	return chores, nil
}

// moveChore relocates the chore file to the directory for newStatus, updates
// its status field, and rewrites it.
func moveChore(p plan, newStatus statusKind) (plan, error) {
	if p.Path == "" {
		return plan{}, fmt.Errorf("move chore: empty path")
	}
	dir := choreDir(newStatus)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return plan{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	newPath := path.Join(dir, path.Base(p.Path))
	if err := os.Rename(p.Path, newPath); err != nil {
		return plan{}, fmt.Errorf("rename %s -> %s: %w", p.Path, newPath, err)
	}
	p.Path = newPath
	p.Status = newStatus
	if err := writePlan(p); err != nil {
		return plan{}, fmt.Errorf("rewrite chore: %w", err)
	}
	return p, nil
}
