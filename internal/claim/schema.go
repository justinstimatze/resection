// Package claim defines the shared schema both cold agents (vision-reader,
// implementation-reader) fill independently, plus the deterministic diff
// over it. See docs/design.md's "Reconciliation" section for why this is a
// typed schema diffed in code rather than a third LLM comparing prose.
package claim

import (
	"fmt"
	"time"
)

type Status string

const (
	StatusDone         Status = "done"
	StatusPartial      Status = "partial"
	StatusNotStarted   Status = "not-started"
	StatusContradicted Status = "contradicted"
)

func (s Status) valid() bool {
	switch s {
	case StatusDone, StatusPartial, StatusNotStarted, StatusContradicted:
		return true
	}
	return false
}

const SchemaVersion = 1

const (
	RoleVision         = "vision"
	RoleImplementation = "implementation"
)

// ClaimStub is what the vision-reader's "enumerate" mode emits: a bare,
// unscored claim. IDs are assigned afterward by `resection assign-ids`,
// never by either LLM — see docs/design.md on why two independent outputs
// shouldn't each invent IDs for "the same" claim.
type ClaimStub struct {
	ID          string `json:"id,omitempty"`
	Description string `json:"description"`
	// Headline marks a claim that states the project's central purpose or
	// uses a named term of art with a specific technical meaning (e.g.
	// "Type-4 detection", "closed-book", "precision floor") — exactly the
	// claims a same-labeled-but-easier substitute hides in. See
	// docs/design.md's "accurate-but-unflagged scope narrowing" paragraph.
	Headline bool `json:"headline"`
}

// ScoredClaim is what each cold agent's "score" mode emits: one claim
// assessed against whichever material that agent was given.
type ScoredClaim struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Headline    bool   `json:"headline"`
	Status      Status `json:"status"`
	// Evidence is a pointer into what the agent actually looked at: a
	// quote/section name (vision side) or a path:line / command + output
	// (implementation side) — never a bare assertion.
	Evidence string `json:"evidence"`
	// FalsificationCriterion is REQUIRED when Headline is true: what would
	// a same-labeled-but-easier substitute look like, and does the
	// evidence actually rule that out? Validate rejects a headline claim
	// with this empty.
	FalsificationCriterion string `json:"falsification_criterion,omitempty"`
	Notes                  string `json:"notes,omitempty"`
}

// Report is the full output of one cold agent's scoring pass.
type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Role          string        `json:"role"`
	Project       string        `json:"project"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Claims        []ScoredClaim `json:"claims"`
}

// Validate checks a Report's structural integrity: correct schema
// version and role, every claim has the fields the diff and the
// headline/falsification-criterion mitigation (docs/design.md's
// "Accurate-but-unflagged scope narrowing" section) both depend on, no
// duplicate IDs, and every headline claim carries a falsification
// criterion. Returns the first
// error found, worded so it can be handed straight back to the
// generating agent as a repair instruction (see skills/resection/SKILL.md
// step 7).
func Validate(r Report, wantRole string) error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if r.Role != wantRole {
		return fmt.Errorf("role %q, want %q", r.Role, wantRole)
	}
	if len(r.Claims) == 0 {
		return fmt.Errorf("claims is empty")
	}
	seen := make(map[string]bool, len(r.Claims))
	for i, c := range r.Claims {
		if c.ID == "" {
			return fmt.Errorf("claim %d: missing id", i)
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
		if c.Description == "" {
			return fmt.Errorf("claim %s: missing description", c.ID)
		}
		if !c.Status.valid() {
			return fmt.Errorf("claim %s: status %q not one of done/partial/not-started/contradicted", c.ID, c.Status)
		}
		if c.Evidence == "" {
			return fmt.Errorf("claim %s: missing evidence", c.ID)
		}
		if c.Headline && c.FalsificationCriterion == "" {
			return fmt.Errorf("claim %s is headline but has no falsification_criterion", c.ID)
		}
	}
	return nil
}
