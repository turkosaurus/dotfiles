package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// confirm shows msg + " [y/N]: " on stderr and reads a y/n answer from stdin.
// Returns true when the user answers yes. Returns true unconditionally when
// confirmYes (--yes) is set. Any read error or empty input is treated as No.
//
// Written to bypass pterm.DefaultInteractiveConfirm, which prints its prompt
// text through pterm's internal stdout path — the shim captures that and the
// user sees nothing until process exit.
func confirm(msg string) bool {
	if confirmYes {
		return true
	}
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", msg)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
