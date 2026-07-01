package main

import (
	"os"
	"path"

	"github.com/pterm/pterm"
)

// log is the structured debug logger. Default level is Info (debug filtered out);
// main flips it to Debug when -v is set. All user-facing output stays on the
// print-style printers (pterm.Info, Success, Warning, Error) for visual parity
// with pickers and tables.
var log = pterm.DefaultLogger.WithLevel(pterm.LogLevelInfo).WithTime(false)

var (
	confirmYes      bool // set from --yes; bypasses confirmation prompts
	defaultWorkDir  = path.Join(os.Getenv("HOME"), "w")
	defaultChoreDir = path.Join(defaultWorkDir, "x")
	defaultDaysDue  = 3
)

func init() {
	// Badge texts padded to 5 chars, shorter words centered.
	pterm.Debug.Prefix.Text = "DEBUG"
	pterm.Info.Prefix.Text = "INFO "
	pterm.Warning.Prefix.Text = "WARN "
	pterm.Error.Prefix.Text = "ERROR"
	pterm.Success.Prefix.Text = " OK  "

	// Route all pterm output to stderr so stdout is reserved for cd-target
	// paths that the shell wrapper consumes.
	pterm.Debug.Writer = os.Stderr
	pterm.Info.Writer = os.Stderr
	pterm.Warning.Writer = os.Stderr
	pterm.Error.Writer = os.Stderr
	pterm.Success.Writer = os.Stderr
	pterm.DefaultSpinner.Writer = os.Stderr
	pterm.DefaultProgressbar.Writer = os.Stderr
}
