package main

import "github.com/pterm/pterm"

type legendCmd struct{}

// runLegend prints the icon set the tool uses (type + status) so the user can
// see which glyphs render in their terminal font.
func runLegend(_ *legendCmd) error {
	rows := pterm.TableData{
		{"glyph", "meaning"},
		{iconWorktree, "worktree"},
		{iconTask, "task"},
		{iconStatusOpen, "status: open"},
		{iconStatusWaiting, "status: waiting"},
		{iconStatusWorking, "status: working"},
		{iconStatusClosed, "status: closed"},
		{iconStatusBroken, "status: broken (plan.toml won't parse)"},
		{iconStatusUnknown, "status: unknown (no plan.toml)"},
	}
	return pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
}
