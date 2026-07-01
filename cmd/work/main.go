package main

import (
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"
)

type args struct {
	Verbose bool `arg:"-v,--verbose" help:"verbose output"`
	Yes     bool `arg:"-y,--yes" help:"assume yes to all prompts"`

	List    *listCmd    `arg:"subcommand:list" help:"list worktrees and chores (alias: ls)"`
	Pick    *pickCmd    `arg:"subcommand:pick" help:"pick a worktree (empty → fzf; name → navigate). same as: work [name]"`
	Main    *mainCmd    `arg:"subcommand:main" help:"switch to the main worktree"`
	Prev    *prevCmd    `arg:"subcommand:-" help:"previous worktree"`
	New     *newCmd     `arg:"subcommand:new" help:"create: worktree from current branch (.), or chore (\"title with spaces\")"`
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
	return "work: manage worktrees, plans, and chores under ~/w"
}

// knownSubcommands lists strings that should NOT be treated as branch names.
// Kept in sync with the arg tags above.
var knownSubcommands = map[string]bool{
	"pick": true, "new": true, "main": true, "-": true,
	"rm": true, "clean": true,
	"list": true, "ls": true, "sync": true, // ls preprocesses to list
	"install": true, "legend": true,
	"help": true, "-h": true, "--help": true,
}

// preprocessArgs rewrites terse forms so go-arg sees a real subcommand:
//   - no args     → "pick"
//   - "help"      → "-h"
//   - "ls"        → "list"
//   - "<branch>"  → "pick <branch>"  (only if <branch> isn't a known subcommand/flag)
//
// `.` and `-` are subcommand names directly (go-arg accepts them) so no
// rewrite is needed for those.
func preprocessArgs() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "pick")
		return
	}
	first := os.Args[1]
	switch first {
	case "help":
		os.Args[1] = "-h"
		return
	case "ls":
		os.Args[1] = "list"
		return
	case ".":
		// muscle-memory alias for `new .`
		os.Args = append([]string{os.Args[0], "new", "."}, os.Args[2:]...)
		return
	}
	if first == "-" {
		// bare dash IS the `prev` subcommand; leave alone.
		return
	}
	if strings.HasPrefix(first, "-") {
		// real flag — leave alone
		return
	}
	if knownSubcommands[first] {
		return
	}
	// branch name — insert "pick" (pick takes an optional positional)
	os.Args = append([]string{os.Args[0], "pick"}, os.Args[1:]...)
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
