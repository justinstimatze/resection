package claim

import (
	"fmt"
	"sort"
	"time"
)

// FlagStatusMismatch is the only flag Diff ever sets. A claim-ID set
// mismatch between the two reports is a pipeline bug, not a finding —
// Diff returns an error for that case instead of a flagged row.
const FlagStatusMismatch = "status_mismatch"

// DiffRow is one claim's side-by-side comparison. Match is the whole
// trust kernel: true iff both sides scored the same Status for the same
// ID. Evidence-text differences never trigger a flag — only status
// disagreement does (docs/design.md: "ensemble agreement is never the
// trust signal" runs backwards here on purpose — disagreement is the signal).
type DiffRow struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	Headline       bool   `json:"headline"`
	VisionStatus   Status `json:"vision_status,omitempty"`
	VisionEvidence string `json:"vision_evidence,omitempty"`
	ImplStatus     Status `json:"impl_status,omitempty"`
	ImplEvidence   string `json:"impl_evidence,omitempty"`
	Match          bool   `json:"match"`
	Flag           string `json:"flag,omitempty"`
}

type DiffResult struct {
	SchemaVersion int       `json:"schema_version"`
	Project       string    `json:"project"`
	GeneratedAt   time.Time `json:"generated_at"`
	Rows          []DiffRow `json:"rows"`
	FlaggedCount  int       `json:"flagged_count"`
}

// Diff compares two Reports claim-by-claim. It returns an error if vision
// isn't actually role "vision", impl isn't actually role "implementation",
// the two reports name different projects, or they don't reference the
// same claim-ID set — every one of those is a pipeline bug (the wrong
// file handed to the wrong flag, or the implementation-reader altering
// the list it was given), not a finding, and callers should treat it as a
// hard failure rather than a disagreement to adjudicate. Without these
// checks, two copies of the same report — or reports for two different
// projects — diff cleanly and report "no disagreement," which is the one
// result this function must never produce by accident.
func Diff(vision, impl Report) (DiffResult, error) {
	if vision.Role != RoleVision {
		return DiffResult{}, fmt.Errorf(
			"vision report has role %q, want %q — refusing to diff: this is not a vision-side report "+
				"(a caller likely passed the same file, or the wrong file, to both --vision and --impl)",
			vision.Role, RoleVision)
	}
	if impl.Role != RoleImplementation {
		return DiffResult{}, fmt.Errorf(
			"impl report has role %q, want %q — refusing to diff: this is not an implementation-side report "+
				"(a caller likely passed the same file, or the wrong file, to both --vision and --impl)",
			impl.Role, RoleImplementation)
	}
	if vision.Project != impl.Project {
		return DiffResult{}, fmt.Errorf(
			"vision report is for project %q but impl report is for project %q — refusing to diff two different projects",
			vision.Project, impl.Project)
	}

	visionByID := make(map[string]ScoredClaim, len(vision.Claims))
	for _, c := range vision.Claims {
		visionByID[c.ID] = c
	}
	implByID := make(map[string]ScoredClaim, len(impl.Claims))
	for _, c := range impl.Claims {
		implByID[c.ID] = c
	}

	var missingInImpl, missingInVision []string
	for id := range visionByID {
		if _, ok := implByID[id]; !ok {
			missingInImpl = append(missingInImpl, id)
		}
	}
	for id := range implByID {
		if _, ok := visionByID[id]; !ok {
			missingInVision = append(missingInVision, id)
		}
	}
	if len(missingInImpl) > 0 || len(missingInVision) > 0 {
		sort.Strings(missingInImpl)
		sort.Strings(missingInVision)
		return DiffResult{}, fmt.Errorf(
			"claim-ID sets don't match: missing in impl %v, missing in vision %v — "+
				"this is a pipeline bug (the implementation-reader altered the claim "+
				"list it was given), not a drift finding",
			missingInImpl, missingInVision)
	}

	ids := make([]string, 0, len(visionByID))
	for id := range visionByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := DiffResult{
		SchemaVersion: SchemaVersion,
		Project:       vision.Project,
		GeneratedAt:   time.Now().UTC(),
		Rows:          make([]DiffRow, 0, len(ids)),
	}
	for _, id := range ids {
		v := visionByID[id]
		im := implByID[id]
		row := DiffRow{
			ID:             id,
			Description:    v.Description,
			Headline:       v.Headline || im.Headline,
			VisionStatus:   v.Status,
			VisionEvidence: v.Evidence,
			ImplStatus:     im.Status,
			ImplEvidence:   im.Evidence,
			Match:          v.Status == im.Status,
		}
		if !row.Match {
			row.Flag = FlagStatusMismatch
			result.FlaggedCount++
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}
