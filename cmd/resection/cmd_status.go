package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/resection/internal/hook"
	"github.com/justinstimatze/resection/internal/store"
)

// hookLogTailLines is how many trailing hook.log lines `resection status`
// prints — enough to see the last few hook fires without dumping a
// potentially 10MB file (hook.Logf's own rotation ceiling) to the
// terminal.
const hookLogTailLines = 20

// cmdStatus implements `resection status [--project P]`: prints the
// counter state for a project (defaulting to cwd) and the tail of
// hook.log, so a human can sanity-check the trigger without reading raw
// JSON files by hand. P is a project root path, resolved through
// store.ResolveProjectArg the same way rundir and the hooks resolve
// theirs — never a pre-resolved identifier passed straight through.
func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	project := fs.String("project", "", "project root path (default: current directory)")
	_ = fs.Parse(args)

	p, err := store.ResolveProjectArg(*project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection status: %s\n", err)
		os.Exit(1)
	}

	dir, err := store.ProjectDir(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection status: %s\n", err)
		os.Exit(1)
	}
	counterPath := filepath.Join(dir, "counter.json")
	data, err := os.ReadFile(counterPath)
	switch {
	case os.IsNotExist(err):
		fmt.Printf("project %q: no counter yet (no PostCompact seen)\n", p)
	case err != nil:
		fmt.Fprintf(os.Stderr, "resection status: %s\n", err)
		os.Exit(1)
	default:
		var pretty map[string]any
		if json.Unmarshal(data, &pretty) == nil {
			out, _ := json.MarshalIndent(pretty, "", "  ")
			fmt.Printf("project %q counter:\n%s\n", p, out)
		} else {
			fmt.Printf("project %q counter (raw): %s\n", p, data)
		}
	}

	logPath, err := hook.LogPath()
	if err != nil {
		return
	}
	tail, err := tailLines(logPath, hookLogTailLines)
	switch {
	case os.IsNotExist(err):
		fmt.Printf("\nhook.log: %s (no entries yet)\n", logPath)
	case err != nil:
		fmt.Fprintf(os.Stderr, "resection status: reading hook.log: %s\n", err)
	case len(tail) == 0:
		fmt.Printf("\nhook.log: %s (empty)\n", logPath)
	default:
		fmt.Printf("\nhook.log: %s (last %d line(s)):\n", logPath, len(tail))
		for _, line := range tail {
			fmt.Println(line)
		}
	}
}

// tailLines returns up to the last n non-empty lines of the file at path.
func tailLines(path string, n int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil, nil
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}
