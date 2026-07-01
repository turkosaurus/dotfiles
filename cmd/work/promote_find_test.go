package main

import (
	"os"
	"path"
	"testing"
)

func TestFindOpenTask(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	defaultWorkDir = path.Join(tmp, "w")
	defaultTaskDir = path.Join(defaultWorkDir, "t")
	openDir := taskDir(statusOpen)
	if err := os.MkdirAll(openDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Two open tasks; one absent.
	for _, n := range []string{"3.toml", "7.toml"} {
		if err := os.WriteFile(path.Join(openDir, n), []byte(""), 0o644); err != nil {
			t.Fatalf("touch %s: %v", n, err)
		}
	}

	tests := []struct {
		n      int
		wantOK bool
	}{
		{3, true},
		{7, true},
		{99, false},
	}
	for _, tc := range tests {
		p, err := findOpenTask(tc.n)
		if tc.wantOK && (err != nil || p == "") {
			t.Errorf("findOpenTask(%d) = (%q, %v); want a path", tc.n, p, err)
		}
		if !tc.wantOK && err == nil {
			t.Errorf("findOpenTask(%d) = (%q, nil); want error", tc.n, p)
		}
	}
}
