package main

import "github.com/pterm/pterm"

func init() {
	pterm.Debug.Prefix.Text = "DEBUG"
	pterm.Info.Prefix.Text = "INFO "
	pterm.Warning.Prefix.Text = "WARN "
	pterm.Error.Prefix.Text = "ERROR"
	pterm.Success.Prefix.Text = "OK   "
}
