package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/justinstimatze/resection/internal/hook"
	"github.com/justinstimatze/resection/internal/store"
)

// defaultN is a placeholder, not a considered choice — the real value is
// picked empirically once v1 has run a few times and there's a
// cost-per-run number to weigh against how often flagged rows turn out to
// matter. Override with RESECTION_N.
const defaultN = 8

func resolveN() int {
	v := os.Getenv("RESECTION_N")
	if v == "" {
		return defaultN
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultN
	}
	return n
}

// sessionStartInput is the subset of SessionStart's hook JSON resection
// reads. Filtering on source=="compact" happens here in Go, not via the
// hook's matcher — SessionStart's matcher has no real filter vocabulary
// for `source` (confirmed against the installed CLI's own embedded
// hooks-reference; only PreCompact/PostCompact match on `trigger`).
type sessionStartInput struct {
	SessionID     string `json:"session_id"`
	CWD           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	Source        string `json:"source"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

type sessionStartOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

const triggerInstruction = "resection: this project has reached its Nth /compact " +
	"since the last alignment check. Run the resection skill now (invoke the " +
	"resection skill) against this project to compare its stated vision against " +
	"its actual implementation. This fires automatically on a cadence; you can " +
	"also run it manually anytime."

// cmdCompactCheck is the SessionStart hook. If this session started
// because of a compact (source=="compact") and the project has reached N
// compacts since the last fire, it prints additionalContext instructing
// Claude to run the resection skill now. Off-Nth or non-compact starts
// print nothing.
func cmdCompactCheck() {
	var in sessionStartInput
	if err := hook.Decode("compact-check", &in); err != nil {
		return
	}
	if in.Source != "compact" {
		return
	}
	if in.CWD == "" {
		hook.Logf("compact-check", "empty cwd, skipping")
		return
	}
	project := store.ResolveProject(in.CWD)
	fired, _, err := store.CheckAndReset(project, resolveN())
	if err != nil {
		hook.Logf("compact-check", "CheckAndReset(%s): %s", project, err)
		return
	}
	if !fired {
		return
	}
	out := sessionStartOutput{HookSpecificOutput: hookSpecificOutput{
		HookEventName:     "SessionStart",
		AdditionalContext: triggerInstruction,
	}}
	data, err := json.Marshal(out)
	if err != nil {
		hook.Logf("compact-check", "marshal: %s", err)
		return
	}
	fmt.Println(string(data))
}
