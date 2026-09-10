package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/justinstimatze/resection/internal/store"
)

// cmdShow implements `resection show [--project P]`: lists past run
// directories for a project (defaulting to cwd), newest first. P is a
// project root path, resolved through store.ResolveProjectArg the same
// way rundir and the hooks resolve theirs — never a pre-resolved
// identifier passed straight through.
func cmdShow(args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	project := fs.String("project", "", "project root path (default: current directory)")
	_ = fs.Parse(args)

	p, err := store.ResolveProjectArg(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection show: %s\n", err)
		os.Exit(1)
	}

	dir, err := store.ProjectDir(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection show: %s\n", err)
		os.Exit(1)
	}
	runsDir := filepath.Join(dir, "runs")
	entries, err := os.ReadDir(runsDir)
	if os.IsNotExist(err) || len(entries) == 0 {
		fmt.Printf("project %q: no runs yet\n", p)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection show: %s\n", err)
		os.Exit(1)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	fmt.Printf("project %q: %d run(s)\n", p, len(names))
	for _, n := range names {
		fmt.Printf("  %s\n", filepath.Join(runsDir, n))
	}
}
