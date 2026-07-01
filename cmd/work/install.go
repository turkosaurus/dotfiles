package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pterm/pterm"
)

type installCmd struct {
	Print bool `arg:"--print" help:"print the shim to stdout instead of appending to rc"`
}

// shellFunc is the shim that must run in the user's shell (bash/zsh) to
// convert the binary's stdout path into a cd. See emitPath for the contract.
// Uses $HOME/go/bin/work so PATH ordering (e.g., an older ~/bin/work) can't
// shadow the Go binary. Non-directory stdout (help text, --print output,
// error messages) is echoed so the user still sees it.
const shellFunc = `# work - git worktree wrapper with cd support
work() {
  local out
  out=$("$HOME/go/bin/work" "$@")
  if [[ -d "$out" ]]; then
    cd "$out"
  elif [[ -n "$out" ]]; then
    printf '%s\n' "$out"
  fi
}
`

// runInstall appends the shell shim to the user's rc, or prints it to stdout
// with --print. Detects the rc from $SHELL (with ZDOTDIR / XDG_CONFIG_HOME
// awareness) and prompts unless confirmYes is set.
func runInstall(c *installCmd) error {
	if c.Print {
		fmt.Print(shellFunc)
		return nil
	}

	shell := path.Base(os.Getenv("SHELL"))
	rc := shellRC(shell)
	if rc == "" {
		return fmt.Errorf("unsupported shell %q; run `work install --print` and paste the output into your rc", shell)
	}

	// Sentinel is the shim's first line (the header comment). Distinctive enough
	// that it won't false-positive against random user comments about work().
	sentinel := strings.SplitN(shellFunc, "\n", 2)[0]
	if already, err := containsLine(rc, sentinel); err != nil {
		return fmt.Errorf("check %s: %w", rc, err)
	} else if already {
		pterm.Success.Printfln("shim already installed in %s", rc)
		return nil
	}

	pterm.Info.Printfln("will append to %s:", rc)
	for _, line := range strings.Split(strings.TrimRight(shellFunc, "\n"), "\n") {
		pterm.Info.Printfln("  %s", line)
	}
	if !confirm("proceed?") {
		return fmt.Errorf("install cancelled")
	}

	f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, planFileMode)
	if err != nil {
		return fmt.Errorf("open %s: %w", rc, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n%s", shellFunc); err != nil {
		return fmt.Errorf("write %s: %w", rc, err)
	}

	pterm.Success.Printfln("appended to %s", rc)
	pterm.Info.Printfln("reload with: source %s", rc)
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
