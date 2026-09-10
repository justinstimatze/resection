package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/resection/internal/store"
)

// cmdRunDir implements `resection rundir [--project PATH]`, printing (and
// creating) a fresh run directory path under
// ~/.claude/resection/projects/<hash>/runs/<timestamp>/ for the skill to
// write vision.json/impl.json/diff.json/report.md into.
//
// PATH is a project root path (default: cwd), resolved through
// store.ResolveProjectArg exactly like a PostCompact/SessionStart hook
// resolves its own cwd — never a pre-resolved identifier passed straight
// through. Passing a raw project path used to hash the path string
// itself, landing in a different bucket than the hooks' own resolution of
// the same project ever would.
func cmdRunDir(args []string) {
	fs := flag.NewFlagSet("rundir", flag.ExitOnError)
	path := fs.String("project", "", "project root path (default: current directory)")
	_ = fs.Parse(args)

	project, err := store.ResolveProjectArg(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection rundir: %s\n", err)
		os.Exit(1)
	}
	dir, err := store.RunDir(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection rundir: %s\n", err)
		os.Exit(1)
	}
	fmt.Println(dir)
}
