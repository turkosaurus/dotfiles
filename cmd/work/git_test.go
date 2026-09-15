package main

import (
	"os/exec"
	"testing"
)

func TestIsGitWorktree(t *testing.T) {
	plain := t.TempDir()
	if isGitWorktree(plain) {
		t.Errorf("isGitWorktree(%q) = true, want false for a plain dir", plain)
	}

	repo := t.TempDir()
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if !isGitWorktree(repo) {
		t.Errorf("isGitWorktree(%q) = false, want true for a git repo", repo)
	}
}
