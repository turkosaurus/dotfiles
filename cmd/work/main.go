package main

import (
	"os"

	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"
)

type args struct {
	Verbose bool `arg:"-v,--verbose" help:"verbose output"`
	Yes     bool `arg:"-y,--yes" help:"assume yes to all prompts"`

	List    *listCmd    `arg:"subcommand:list" help:"list worktrees and tasks (alias: ls)"`
	Pick    *pickCmd    `arg:"subcommand:pick" help:"pick a worktree (empty → fzf; name → navigate). same as: work [name]"`
	Main    *mainCmd    `arg:"subcommand:main" help:"switch to the main worktree"`
	Prev    *prevCmd    `arg:"subcommand:-" help:"previous worktree"`
	New     *newCmd     `arg:"subcommand:new" help:"create: worktree from current branch (.), or task (\"title with spaces\")"`
	Clean   *cleanCmd   `arg:"subcommand:clean" help:"remove worktrees with merged/closed PRs"`
	Rm      *rmCmd      `arg:"subcommand:rm" help:"remove a worktree"`
	Sync    *syncCmd    `arg:"subcommand:sync" help:"sync plan with github"`
	Install *installCmd `arg:"subcommand:install" help:"append the shim to your shellrc (--print to stdout instead)"`
	Legend  *legendCmd  `arg:"subcommand:legend" help:"print the icon legend"`
}

type syncCmd struct {
	All bool `arg:"-a,--all" help:"sync all plans"` // TODO: use
}

func (args) Description() string {
	return "work: manage worktrees, plans, and tasks under ~/w"
}

// knownSubcommands lists tokens go-arg recognizes as a subcommand name.
// Anything else in that position becomes `pick <arg>` via preprocessArgs.
var knownSubcommands = map[string]bool{
	"pick": true, "new": true, "main": true, "-": true,
	"rm": true, "clean": true, "list": true, "sync": true,
	"install": true, "legend": true,
}

// globalFlags are the top-level flags that must precede a subcommand.
var globalFlags = map[string]bool{
	"-v": true, "--verbose": true,
	"-y": true, "--yes": true,
	"-h": true, "--help": true,
}

// preprocessArgs rewrites terse forms so go-arg sees a real subcommand:
//   - no args              → "pick"
//   - only globals         → "pick" (e.g., `work -v`)
//   - "help"               → "-h"
//   - "ls"                 → "list"
//   - "."                  → "new ."
//   - "<branch>"           → "pick <branch>"
//   - "-c" / "-w" / etc.   → "pick -c" / "pick -w" (subcommand-scoped flag → pick)
//
// `-` (bare dash) is the `prev` subcommand; left alone.
func preprocessArgs() {
	// Skip past global flags.
	i := 1
	for i < len(os.Args) && globalFlags[os.Args[i]] {
		i++
	}
	if i >= len(os.Args) {
		os.Args = append(os.Args, "pick")
		return
	}
	tok := os.Args[i]

	switch tok {
	case "help":
		os.Args[i] = "-h"
		return
	case "ls":
		os.Args[i] = "list"
		return
	case ".":
		out := append([]string(nil), os.Args[:i]...)
		out = append(out, "new", ".")
		out = append(out, os.Args[i+1:]...)
		os.Args = out
		return
	case "-":
		return // bare dash IS the `prev` subcommand
	}
	if knownSubcommands[tok] {
		return
	}
	// Either a branch positional or a subcommand-scoped flag (e.g., -c).
	// Insert "pick" here so go-arg parses it as a pickCmd arg.
	out := append([]string(nil), os.Args[:i]...)
	out = append(out, "pick")
	out = append(out, os.Args[i:]...)
	os.Args = out
}

func main() {
	preprocessArgs()

	var a args
	arg.MustParse(&a)

	if a.Verbose {
		log = log.WithLevel(pterm.LogLevelDebug)
	}
	confirmYes = a.Yes
	log.Debug("verbose mode enabled")

	if err := setupDirs(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	if err := dispatch(&a); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}

func dispatch(a *args) error {
	switch {
	case a.Pick != nil:
		return runPick(a.Pick)
	case a.New != nil:
		return runNew(a.New)
	case a.Main != nil:
		return runMain(a.Main)
	case a.Prev != nil:
		return runPrev(a.Prev)
	case a.Rm != nil:
		return runRm(a.Rm)
	case a.Clean != nil:
		return runClean(a.Clean)
	case a.List != nil:
		return runList(a.List)
	case a.Sync != nil:
		return runSync(a.Sync)
	case a.Install != nil:
		return runInstall(a.Install)
	case a.Legend != nil:
		return runLegend(a.Legend)
	}
	// Fallthrough (shouldn't happen: preprocessArgs inserts "pick" for no-args).
	return runPick(&pickCmd{})
}
