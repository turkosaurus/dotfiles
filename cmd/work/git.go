package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// repoRoot returns the toplevel directory of the git repo containing dir.
func repoRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// currentBranch returns the checked-out branch name for the worktree at dir.
func currentBranch(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("git branch: %w", err)
	}
	b := strings.TrimSpace(string(out))
	if b == "" {
		return "", fmt.Errorf("no branch (detached HEAD?)")
	}
	return b, nil
}

// originOwnerRepo parses the origin remote URL into owner and repo.
// Handles ssh (git@github.com:o/r) and https (https://github.com/o/r) forms.
func originOwnerRepo(dir string) (owner, repo string, err error) {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", fmt.Errorf("git remote: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	raw = strings.TrimSuffix(raw, ".git")

	if strings.HasPrefix(raw, "git@") {
		_, after, ok := strings.Cut(raw, ":")
		if !ok {
			return "", "", fmt.Errorf("bad ssh url %q", raw)
		}
		parts := strings.SplitN(after, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("bad ssh url %q", raw)
		}
		return parts[0], parts[1], nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse url %q: %w", raw, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("bad url %q", raw)
	}
	return parts[0], parts[1], nil
}
