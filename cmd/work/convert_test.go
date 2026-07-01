package main

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestConvertPlanToTaskIfPending(t *testing.T) {
	// Isolate HOME so ~/w/t/... points into a scratch dir.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Rebind defaults (they were computed at package init time from the old HOME).
	defaultWorkDir = path.Join(tmp, "w")
	defaultTaskDir = path.Join(defaultWorkDir, "t")
	for _, s := range []statusKind{statusOpen, statusWaiting, statusWorking, statusClosed} {
		if err := os.MkdirAll(taskDir(s), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(path.Join(defaultTaskDir, taskCounterFile), []byte("42\n"), 0o644); err != nil {
		t.Fatalf("counter: %v", err)
	}

	// Fake worktree with a plan.toml carrying tasks.
	wtDir := path.Join(defaultWorkDir, "myrepo", "branch")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	p := plan{
		Title:  "my branch",
		Status: statusWorking,
		Due:    time.Date(2026, 7, 4, 0, 0, 0, 0, time.Local),
		Tasks:  []string{"task one", "task two"},
		Path:   path.Join(wtDir, planFileName),
	}
	if err := writePlan(p); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	// Case 1: has tasks, status=working → conversion preserves status,
	// so the task lands in ~/w/t/working/42.toml (not open/).
	newPath, err := convertPlanToTaskIfPending(worktree{Path: wtDir})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	want := path.Join(taskDir(statusWorking), "42.toml")
	if newPath != want {
		t.Errorf("newPath = %q, want %q", newPath, want)
	}
	got, err := readPlan(newPath)
	if err != nil {
		t.Fatalf("read converted: %v", err)
	}
	if got.Status != statusWorking {
		t.Errorf("status = %q, want working", got.Status)
	}
	if got.Title != "my branch" || len(got.Tasks) != 2 {
		t.Errorf("plan not preserved: %+v", got)
	}
	if !strings.HasSuffix(got.Path, "/42.toml") {
		t.Errorf("path field = %q, want …/42.toml", got.Path)
	}

	// Case 2: empty tasks → no-op.
	wtDir2 := path.Join(defaultWorkDir, "myrepo", "empty")
	if err := os.MkdirAll(wtDir2, 0o755); err != nil {
		t.Fatalf("mkdir wt2: %v", err)
	}
	p.Tasks = nil
	p.Path = path.Join(wtDir2, planFileName)
	if err := writePlan(p); err != nil {
		t.Fatalf("write plan 2: %v", err)
	}
	newPath2, err := convertPlanToTaskIfPending(worktree{Path: wtDir2})
	if err != nil || newPath2 != "" {
		t.Errorf("empty-tasks case: got (%q, %v), want (\"\", nil)", newPath2, err)
	}

	// Case 3: missing plan.toml → no-op.
	newPath3, err := convertPlanToTaskIfPending(worktree{Path: path.Join(defaultWorkDir, "nope")})
	if err != nil || newPath3 != "" {
		t.Errorf("missing-plan case: got (%q, %v), want (\"\", nil)", newPath3, err)
	}

	// Case 4: status=closed with tasks still populated → warn+confirm, then
	// land in ~/w/t/working/ (not closed/), so almost-orphaned tasks surface
	// as active work instead of vanishing into the closed pile.
	origYes := confirmYes
	confirmYes = true
	t.Cleanup(func() { confirmYes = origYes })

	wtDir4 := path.Join(defaultWorkDir, "myrepo", "closed-with-tasks")
	if err := os.MkdirAll(wtDir4, 0o755); err != nil {
		t.Fatalf("mkdir wt4: %v", err)
	}
	p4 := plan{
		Title:  "closed but tasky",
		Status: statusClosed,
		Due:    time.Date(2026, 7, 4, 0, 0, 0, 0, time.Local),
		Tasks:  []string{"still open"},
		Path:   path.Join(wtDir4, planFileName),
	}
	if err := writePlan(p4); err != nil {
		t.Fatalf("write plan 4: %v", err)
	}
	newPath4, err := convertPlanToTaskIfPending(worktree{Path: wtDir4})
	if err != nil {
		t.Fatalf("closed+tasks (yes): %v", err)
	}
	if !strings.Contains(newPath4, "/working/") {
		t.Errorf("closed+tasks should convert to working; got %q", newPath4)
	}
}
