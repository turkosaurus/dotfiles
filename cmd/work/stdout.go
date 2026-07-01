package main

import (
	"os"
	"syscall"
)

// realStdout is the process's original fd-1 target, captured before we point
// fd 1 at fd 2. It's the only writer that reaches the shell wrapper (which
// captures the tool's stdout via $() for cd navigation).
//
// Every other write — pterm badges, spinners, interactive selects, cursor
// escape sequences from atomicgo/cursor, plain fmt.Println, etc. — ends up on
// the terminal (via what was stderr), because they all eventually route
// through fd 1, which is now a duplicate of fd 2.
//
// This is the only reliable way to redirect *all* pterm output: setting
// per-printer Writer fields covers static outputs, but atomicgo/cursor
// captures os.Stdout at its package-init time and won't respect later
// reassignments of the os.Stdout variable.
var realStdout *os.File

func init() {
	dup, err := syscall.Dup(int(os.Stdout.Fd()))
	if err != nil {
		return
	}
	realStdout = os.NewFile(uintptr(dup), "real-stdout")
	if err := syscall.Dup2(int(os.Stderr.Fd()), int(os.Stdout.Fd())); err != nil {
		// Best-effort: if dup2 fails, we still have realStdout but the
		// terminal will see os.Stdout writes go to the shim's captured pipe.
		return
	}
}
