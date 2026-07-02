package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type installCmd struct {
	Print    bool   `arg:"--print" help:"print the shim to stdout and exit; no build, no rc edit"`
	ShimOnly bool   `arg:"--shim-only" help:"install/refresh the rc shim only; skip building the binary"`
	From     string `arg:"--from" help:"source directory to build from (contains cmd/work). Persisted for future runs."`
}

// resolveBinaryTarget returns the path where `go install` would write the
// binary, matching Go's own precedence: GOBIN → GOPATH/bin → $HOME/go/bin.
// If `go env` fails (unusual), falls back to $HOME/go/bin.
func resolveBinaryTarget() string {
	if out, err := exec.Command("go", "env", "GOBIN").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return path.Join(p, "work")
		}
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return path.Join(p, "bin", "work")
		}
	}
	return path.Join(os.Getenv("HOME"), "go", "bin", "work")
}

// sourcePathFile is the pre-config.toml single-line file that used to hold
// the source directory. Kept for one-shot migration in loadConfig.
func sourcePathFile() string {
	return path.Join(xdgConfigDir("work"), "source-path")
}

// shellFuncHeader is the sentinel line used to detect a pre-existing install.
// Kept out of the template so a template edit doesn't silently change it.
const shellFuncHeader = "# work - git worktree wrapper with cd support"

// shellBinaryMarker is a stable "key: value" comment right below the shim
// header — readShimTarget parses this to recover the previously-installed
// binary path across re-installs. Kept separate from the shell body so
// changes to the function shape don't break detection.
const shellBinaryMarker = "# work-binary: "

// shellFuncTemplate renders the shim with the resolved binary path baked in.
// The tool writes its next cd-target to $HOME/w/.next; stdout stays free for
// pipes and grep. The shim clears .next before the run (so a crash from a
// previous invocation doesn't leak into this one), then cd's iff .next was
// written with a valid directory.
const shellFuncTemplate = shellFuncHeader + `
` + shellBinaryMarker + `%[1]s
work() {
  local next="$HOME/w/.next"
  rm -f "$next"
  %[1]q "$@"
  local rc=$?
  if [[ -s "$next" ]]; then
    local target
    target=$(cat "$next")
    rm -f "$next"
    [[ -d "$target" ]] && cd "$target"
  fi
  return $rc
}
`

func renderShim(binaryPath string) string {
	return fmt.Sprintf(shellFuncTemplate, binaryPath)
}

// runInstall performs (in order):
//  1. optional binary build → <target>/work  (unless --shim-only)
//  2. shim install/refresh into the user's rc  (unless --print)
//
// Target-path precedence: existing shim path (if present) → `go env GOBIN` →
// `go env GOPATH/bin` → $HOME/go/bin. Honoring an existing shim keeps a
// re-install idempotent even when GOBIN has since diverged.
//
// --print short-circuits everything and dumps the shim to stdout.
func runInstall(c *installCmd) error {
	shell := path.Base(os.Getenv("SHELL"))
	rc := shellRC(shell)

	target := ""
	if rc != "" {
		target = readShimTarget(rc)
	}
	if target == "" {
		target = resolveBinaryTarget()
	}
	shim := renderShim(target)

	if c.Print {
		fmt.Print(shim)
		return nil
	}

	if !c.ShimOnly {
		src, err := resolveSource(c.From)
		if err != nil {
			return fmt.Errorf("install: %w", err)
		}
		if err := buildBinary(src, target); err != nil {
			return fmt.Errorf("install: build: %w", err)
		}
		if err := persistSource(src); err != nil {
			pterm.Warning.Printfln("could not persist source path: %v", err)
		}
		pterm.Success.Printfln("built %s from %s", target, src)
	}

	if rc == "" {
		return fmt.Errorf("unsupported shell %q; run `work install --print` and paste the output into your rc", shell)
	}

	written, err := writeShim(rc, shim)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	switch written {
	case shimUnchanged:
		pterm.Success.Printfln("shim in %s already current", rc)
	case shimReplaced:
		pterm.Success.Printfln("shim in %s updated", rc)
		pterm.Info.Printfln("reload with: source %s", rc)
	case shimAppended:
		pterm.Success.Printfln("shim appended to %s", rc)
		pterm.Info.Printfln("reload with: source %s", rc)
	}
	return nil
}

// shim block replacement outcomes for reporting.
type shimResult int

const (
	shimUnchanged shimResult = iota
	shimReplaced
	shimAppended
)

// writeShim ensures rc contains exactly the given shim block. If a block
// starting with shellFuncHeader is already present, it is replaced (bytes
// from the header line through the terminating `}` line). Otherwise the
// block is appended.
func writeShim(rc, shim string) (shimResult, error) {
	data, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", rc, err)
	}
	existing := string(data)
	start, end, found := findShimBlock(existing)
	if !found {
		f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, planFileMode)
		if err != nil {
			return 0, fmt.Errorf("open %s: %w", rc, err)
		}
		defer f.Close()
		if _, err := fmt.Fprintf(f, "\n%s", shim); err != nil {
			return 0, fmt.Errorf("write %s: %w", rc, err)
		}
		return shimAppended, nil
	}
	if existing[start:end] == shim {
		return shimUnchanged, nil
	}
	newContent := existing[:start] + shim + existing[end:]
	if err := os.WriteFile(rc, []byte(newContent), planFileMode); err != nil {
		return 0, fmt.Errorf("write %s: %w", rc, err)
	}
	return shimReplaced, nil
}

// findShimBlock locates the shim block bounded by shellFuncHeader and the
// next `}` line. Returns byte offsets and whether it was found.
func findShimBlock(rc string) (start, end int, found bool) {
	i := strings.Index(rc, shellFuncHeader)
	if i < 0 {
		return 0, 0, false
	}
	// End is the first `}` on its own line after the header.
	rest := rc[i:]
	closeIdx := strings.Index(rest, "\n}\n")
	if closeIdx < 0 {
		// Handle trailing `}` at EOF without newline.
		if strings.HasSuffix(rest, "\n}") {
			return i, i + len(rest), true
		}
		return 0, 0, false
	}
	return i, i + closeIdx + len("\n}\n"), true
}

// readShimTarget extracts the binary path from an existing shim block in
// rc. The path is read from the `# work-binary: <path>` marker line
// (stable across shell body edits). Returns "" if no shim is present or
// the marker is missing.
func readShimTarget(rc string) string {
	data, err := os.ReadFile(rc)
	if err != nil {
		return ""
	}
	s, e, ok := findShimBlock(string(data))
	if !ok {
		return ""
	}
	block := string(data)[s:e]
	for _, line := range strings.Split(block, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), shellBinaryMarker); ok {
			return os.ExpandEnv(strings.TrimSpace(v))
		}
	}
	return ""
}

// resolveSource decides which module directory to build from, in priority
// order: --from flag → persisted path → cwd. Each candidate is normalized to
// the module dir (containing go.mod) — the caller may pass either the repo
// root or the module dir; both work.
func resolveSource(from string) (string, error) {
	if from != "" {
		mod, err := findModule(from)
		if err != nil {
			return "", fmt.Errorf("--from %s: %w", from, err)
		}
		return mod, nil
	}
	if p := loadPersistedSource(); p != "" {
		if mod, err := findModule(p); err == nil {
			return mod, nil
		}
		pterm.Warning.Printfln("persisted source %s no longer valid; falling back", p)
	}
	if cwd, err := os.Getwd(); err == nil {
		if mod, err := findModule(cwd); err == nil {
			return mod, nil
		}
	}
	return "", fmt.Errorf("no source found: pass --from <repo-or-module-path> once (persists for next time)")
}

// findModule normalizes a user-supplied path to the module directory (the one
// containing go.mod). Accepts either the repo root (…/dotfiles/work) or the
// module dir itself (…/dotfiles/work/cmd/work).
func findModule(dir string) (string, error) {
	if _, err := os.Stat(path.Join(dir, "go.mod")); err == nil {
		return dir, nil
	}
	if p := path.Join(dir, "cmd", "work"); pathExists(path.Join(p, "go.mod")) {
		return p, nil
	}
	return "", fmt.Errorf("no go.mod at %s or %s/cmd/work", dir, dir)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// buildBinary runs `go build -o <target> .` inside the module dir.
// Streams go's stderr through so a build failure is visible.
func buildBinary(moduleDir, target string) error {
	if err := os.MkdirAll(path.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", path.Dir(target), err)
	}
	cmd := exec.Command("go", "build", "-o", target, ".")
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	pterm.Info.Printfln("building %s ← %s", target, moduleDir)
	return cmd.Run()
}

// loadPersistedSource returns the previously-saved source path from the
// config file, or "" if none is set. Falls back to the legacy single-line
// source-path file when the config doesn't have a value yet.
func loadPersistedSource() string {
	c, err := loadConfig()
	if err != nil {
		return ""
	}
	return c.Source.Path
}

// persistSource stores src in config.source.path. Always writes so a missing
// config.toml is created; the legacy single-line source-path file is removed
// on best-effort basis once the config is in place.
func persistSource(src string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	c.Source.Path = src
	if err := saveConfig(c); err != nil {
		return err
	}
	_ = os.Remove(sourcePathFile()) // legacy file, best-effort cleanup
	return nil
}

// shellRC returns the interactive rc path for the given shell name, honoring
// zsh's ZDOTDIR and XDG_CONFIG_HOME conventions.
func shellRC(shell string) string {
	home := os.Getenv("HOME")
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = path.Join(home, ".config")
	}

	switch shell {
	case "zsh":
		// zsh's canonical override: ZDOTDIR
		if zdot := os.Getenv("ZDOTDIR"); zdot != "" {
			return path.Join(zdot, ".zshrc")
		}
		// XDG-style layout many people use
		if p := path.Join(xdg, "zsh", ".zshrc"); fileExists(p) {
			return p
		}
		return path.Join(home, ".zshrc")
	case "bash":
		// XDG-style layout (non-standard but common)
		if p := path.Join(xdg, "bash", "bashrc"); fileExists(p) {
			return p
		}
		return path.Join(home, ".bashrc")
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// containsLine checks whether the file at p contains a line matching the given
// text after both sides are trimmed. Whole-line match (no substring), so a
// distinctive sentinel like the shim header won't false-positive.
func containsLine(p, needle string) (bool, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	needle = strings.TrimSpace(needle)
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) == needle {
			return true, nil
		}
	}
	return false, nil
}
