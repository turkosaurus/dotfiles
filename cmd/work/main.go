package main

import (
	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"
)

type args struct {
	Verbose bool `arg:"-v,--verbose" help:"verbose output"`
}

func (args) Description() string {
	return "work: manage plan.toml files for chores and follow-ups"
}

func main() {
	var a args
	arg.MustParse(&a)

	if a.Verbose {
		pterm.EnableDebugMessages()
	}

	pterm.Debug.Println("verbose mode enabled")
	pterm.Info.Println("hello world")
}
