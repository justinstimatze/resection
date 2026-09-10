package claim

import (
	"encoding/json"
	"fmt"
	"sort"
)

// MergeDescriptions re-attaches each claim's description from the
// original claim list into a scored Report's raw JSON, keyed by id.
//
// Both cold-agent prompts hand each claim its description already
// assigned by `resection assign-ids` — asking either agent to faithfully
// echo it back is redundant token cost and a needless source of
// transcription drift. The vision-reader's score-mode dispatch has
// dropped description from every claim in both real runs against this
// project so far. This always trusts claimsJSON over whatever the agent
// did or didn't include, so it's correct whether the agent omitted the
// field or echoed a subtly wrong one.
//
// Every other field, including a malformed schema_version, passes
// through untouched — this fixes description only, deliberately not the
// separate type-format issue `resection validate` already catches on its
// own.
//
// It also errors if any claim from claimsJSON never appears among the
// scored claims — an agent that scores a subset must not silently
// produce a report that later validates and diffs cleanly.
func MergeDescriptions(claimsJSON, scoredJSON []byte) ([]byte, error) {
	var stubs []ClaimStub
	if err := json.Unmarshal(claimsJSON, &stubs); err != nil {
		return nil, fmt.Errorf("parsing claims list: %w", err)
	}
	descByID := make(map[string]string, len(stubs))
	for _, s := range stubs {
		descByID[s.ID] = s.Description
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal(scoredJSON, &report); err != nil {
		return nil, fmt.Errorf("parsing scored report: %w", err)
	}
	rawClaims, ok := report["claims"]
	if !ok {
		return nil, fmt.Errorf(`scored report has no "claims" field`)
	}
	var claims []map[string]json.RawMessage
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return nil, fmt.Errorf("parsing scored report's claims array: %w", err)
	}

	seen := make(map[string]bool, len(claims))
	for i, c := range claims {
		idRaw, ok := c["id"]
		if !ok {
			return nil, fmt.Errorf("claim %d: missing id, cannot attach description", i)
		}
		var id string
		if err := json.Unmarshal(idRaw, &id); err != nil {
			return nil, fmt.Errorf("claim %d: id is not a string: %w", i, err)
		}
		desc, ok := descByID[id]
		if !ok {
			return nil, fmt.Errorf("claim %s: not present in the original claims list", id)
		}
		seen[id] = true
		descJSON, err := json.Marshal(desc)
		if err != nil {
			return nil, err
		}
		claims[i]["description"] = descJSON
	}

	// The reverse check: every claim the agent was HANDED must come back
	// SCORED. Without this, an agent that silently drops a claim (both
	// real runs against this project have hit some version of "agent
	// drops a field on echo") produces a report that still validates and
	// still diffs cleanly against a same-sized drop on the other side —
	// a confidently clean result over a subset of the claim list, with no
	// signal anywhere that anything is missing.
	var missing []string
	for id := range descByID {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf(
			"claim(s) %v are in the original claims list but were never scored — "+
				"the agent must score every claim it was given, not a subset", missing)
	}

	mergedClaims, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}
	report["claims"] = mergedClaims

	return json.MarshalIndent(report, "", "  ")
}
