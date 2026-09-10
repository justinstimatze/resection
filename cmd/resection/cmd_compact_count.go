package main

import (
	"github.com/justinstimatze/resection/internal/hook"
	"github.com/justinstimatze/resection/internal/store"
)

// postCompactInput is the subset of PostCompact's hook JSON resection
// reads. trigger ("manual" | "auto") and compact_summary are accepted but
// not filtered on — both kinds of compact count toward N.
type postCompactInput struct {
	SessionID      string `json:"session_id"`
	CWD            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Trigger        string `json:"trigger"`
	CompactSummary string `json:"compact_summary"`
	TranscriptPath string `json:"transcript_path"`
}

// cmdCompactCount is the PostCompact hook. Pure side effect: it never
// prints anything, because PostCompact has no hookSpecificOutput channel
// in the SDK's type union at all (confirmed directly against the
// installed Claude Agent SDK's type declarations, not assumed from
// summarized docs). Any error is logged, never surfaced; the hook always
// exits 0.
func cmdCompactCount() {
	var in postCompactInput
	if err := hook.Decode("compact-count", &in); err != nil {
		return
	}
	if in.CWD == "" {
		hook.Logf("compact-count", "empty cwd, skipping")
		return
	}
	project := store.ResolveProject(in.CWD)
	if err := store.IncrementCompact(project); err != nil {
		hook.Logf("compact-count", "IncrementCompact(%s): %s", project, err)
	}
}
