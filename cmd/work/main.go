package main

import (
	"os"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"
)

type args struct {
	Verbose bool   `arg:"-v,--verbose" help:"verbose output"`
	Quiet   bool   `arg:"-q,--quiet" help:"suppress INFO and SUCCESS output; only WARN and ERROR appear"`
	Yes     bool   `arg:"-y,--yes" help:"assume yes to all prompts"`
	Project string `arg:"-p,--project" help:"filter every picker/list to items {t,true} that have a linked issue with a project, or {f,false} to items without"`

	List    *listCmd    `arg:"subcommand:list" help:"list worktrees and tasks (alias: ls)"`
	Pick    *pickCmd    `arg:"subcommand:pick" help:"pick a worktree (empty → fzf; name → navigate). same as: work [name]"`
	Main    *mainCmd    `arg:"subcommand:main" help:"switch to the main worktree"`
	Prev    *prevCmd    `arg:"subcommand:-" help:"previous worktree"`
	New     *newCmd     `arg:"subcommand:new" help:"create: worktree from current branch (.), or task (\"title with spaces\")"`
	Status  *statusCmd  `arg:"subcommand:status" help:"multiselect + set status: -o/-w/-W/-c; -t/-b to narrow (alias: set)"`
	Edit    *editCmd    `arg:"subcommand:edit" help:"edit current worktree's plan.toml (default); -a for batch status editor"`
	Clean   *cleanCmd   `arg:"subcommand:clean" help:"remove worktrees with merged/closed PRs"`
	Rm      *rmCmd      `arg:"subcommand:rm" help:"remove a worktree"`
	Promote *promoteCmd `arg:"subcommand:promote" help:"multiselect one or more tasks; fold them into a new worktree from the current branch"`
	Sync    *syncCmd    `arg:"subcommand:sync" help:"sync plan with github"`
	Install *installCmd `arg:"subcommand:install" help:"append the shim to your shellrc (--print to stdout instead)"`
	Legend   *legendCmd   `arg:"subcommand:legend" help:"print the icon legend"`
	Validate *validateCmd `arg:"subcommand:validate" help:"parse current worktree's plan.toml (default); -a for every plan"`
	Config   *configCmd   `arg:"subcommand:config" help:"open $XDG_CONFIG_HOME/work/config.toml in $EDITOR (seeded with defaults + comments)"`
	Merge    *mergeCmd    `arg:"subcommand:merge" help:"merge multiple plans (worktrees + tasks) into one — auto-picks primary, unions tasks[] + [[issue]]"`
}

type configCmd struct{}

type mergeCmd struct{}

type syncCmd struct {
	All    bool `arg:"-a,--all" help:"sync every worktree plan (default: current worktree only)"`
	DryRun bool `arg:"-d,--dry-run" help:"preview sprint changes (creates/status updates) without writing"`
}

func (args) Description() string {
	return "work: manage worktrees, plans, and tasks under ~/w"
}

// knownSubcommands lists tokens go-arg recognizes as a subcommand name.
// Anything else in that position becomes `pick <arg>` via preprocessArgs.
var knownSubcommands = map[string]bool{
	"pick": true, "new": true, "main": true, "-": true,
	"status": true, "set": true, "edit": true, "rm": true, "clean": true, "list": true, "sync": true,
	"install": true, "legend": true, "validate": true, "promote": true, "config": true, "merge": true,
}

// globalFlags are the top-level flags that must precede a subcommand.
// -p/--project takes a value (t/true/f/false); the picker filter is
// applied in loadInventory via the projectFilter package-level var.
var globalFlags = map[string]bool{
	"-v": true, "--verbose": true,
	"-q": true, "--quiet": true,
	"-y": true, "--yes": true,
	"-h": true, "--help": true,
	"-p": true, "--project": true,
}

// splitBundledShorts turns POSIX-style bundled shorts (like -bWy) into
// individual flags (-b -W -y) so go-arg can parse them. Assumes all short
// flags are boolean (safe today — we have no value-taking shorts).
// Preserves os.Args[0] and leaves long flags (--foo) and value-carrying
// arguments (--k=v, -k=v) untouched.
func splitBundledShorts() {
	out := make([]string, 0, len(os.Args))
	out = append(out, os.Args[0])
	for _, a := range os.Args[1:] {
		if len(a) > 2 && strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && !strings.ContainsRune(a, '=') {
			for _, r := range a[1:] {
				out = append(out, "-"+string(r))
			}
			continue
		}
		out = append(out, a)
	}
	os.Args = out
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
	// Skip past global flags. Handles the three shapes go-arg accepts:
	//   -v                 (bool)
	//   -p=value / --project=value
	//   -p value           (need to skip the following token too)
	valueTaking := map[string]bool{"-p": true, "--project": true}
	i := 1
	for i < len(os.Args) {
		tok := os.Args[i]
		// key=value form
		if eq := strings.IndexByte(tok, '='); eq > 0 {
			key := tok[:eq]
			if globalFlags[key] {
				i++
				continue
			}
			break
		}
		if !globalFlags[tok] {
			break
		}
		i++
		// value-taking global consumes the next token as its argument
		if valueTaking[tok] && i < len(os.Args) {
			i++
		}
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
	case "set":
		os.Args[i] = "status"
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
	splitBundledShorts()
	preprocessArgs()

	var a args
	arg.MustParse(&a)

	if a.Verbose {
		log = log.WithLevel(pterm.LogLevelDebug)
	}
	if a.Quiet {
		setQuietMode()
	}
	confirmYes = a.Yes
	if err := setProjectFilter(a.Project); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
	log.Debug("verbose mode enabled")

	if err := setupDirs(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	if err := dispatch(&a); err != nil {
		// Empty message = the subcommand already printed. Non-empty = summary
		// exit reason; print once here.
		if err.Error() != "" {
			pterm.Error.Println(err)
		}
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
	case a.Status != nil:
		return runStatus(a.Status)
	case a.Edit != nil:
		return runEdit(a.Edit)
	case a.Rm != nil:
		return runRm(a.Rm)
	case a.Promote != nil:
		return runPromote(a.Promote)
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
	case a.Validate != nil:
		return runValidate(a.Validate)
	case a.Config != nil:
		return runConfig(a.Config)
	case a.Merge != nil:
		return runMerge(a.Merge)
	}
	// Fallthrough (shouldn't happen: preprocessArgs inserts "pick" for no-args).
	return runPick(&pickCmd{})
}
