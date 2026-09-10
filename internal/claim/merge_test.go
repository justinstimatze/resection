package claim

import (
	"encoding/json"
	"strings"
	"testing"
)

const testClaimsJSON = `[
  {"id": "claim-01", "description": "First claim.", "headline": false},
  {"id": "claim-02", "description": "Second claim.", "headline": true}
]`

func TestMergeDescriptions_attachesMissingField(t *testing.T) {
	scored := `{
  "schema_version": 1,
  "role": "vision",
  "project": "p",
  "generated_at": "2026-01-01T00:00:00Z",
  "claims": [
    {"id": "claim-01", "status": "done", "evidence": "e1"},
    {"id": "claim-02", "status": "done", "evidence": "e2", "falsification_criterion": "f2"}
  ]
}`
	out, err := MergeDescriptions([]byte(testClaimsJSON), []byte(scored))
	if err != nil {
		t.Fatalf("MergeDescriptions: %v", err)
	}

	var got Report
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-parsing merged output: %v", err)
	}
	if len(got.Claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(got.Claims))
	}
	if got.Claims[0].Description != "First claim." {
		t.Fatalf("claim-01 description = %q, want %q", got.Claims[0].Description, "First claim.")
	}
	if got.Claims[1].Description != "Second claim." {
		t.Fatalf("claim-02 description = %q, want %q", got.Claims[1].Description, "Second claim.")
	}
}

func TestMergeDescriptions_overwritesWrongExisting(t *testing.T) {
	scored := `{
  "schema_version": 1,
  "role": "implementation",
  "project": "p",
  "generated_at": "2026-01-01T00:00:00Z",
  "claims": [
    {"id": "claim-01", "description": "an agent's own paraphrase, not the original", "status": "done", "evidence": "e1"},
    {"id": "claim-02", "status": "done", "evidence": "e2", "falsification_criterion": "f2"}
  ]
}`
	out, err := MergeDescriptions([]byte(testClaimsJSON), []byte(scored))
	if err != nil {
		t.Fatalf("MergeDescriptions: %v", err)
	}
	var got Report
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-parsing merged output: %v", err)
	}
	if got.Claims[0].Description != "First claim." {
		t.Fatalf("expected the original claims-list description to win, got %q", got.Claims[0].Description)
	}
}

func TestMergeDescriptions_preservesMalformedSchemaVersion(t *testing.T) {
	// A string schema_version is exactly the real, reproduced bug this
	// merge step must NOT paper over — resection validate needs to keep
	// catching it independently.
	scored := `{
  "schema_version": "1.0",
  "role": "vision",
  "project": "p",
  "generated_at": "2026-01-01T00:00:00Z",
  "claims": [
    {"id": "claim-01", "status": "done", "evidence": "e1"},
    {"id": "claim-02", "status": "done", "evidence": "e2", "falsification_criterion": "f2"}
  ]
}`
	out, err := MergeDescriptions([]byte(testClaimsJSON), []byte(scored))
	if err != nil {
		t.Fatalf("MergeDescriptions: %v", err)
	}
	if !strings.Contains(string(out), `"schema_version": "1.0"`) {
		t.Fatalf("expected the malformed schema_version to pass through untouched, got: %s", out)
	}
}

func TestMergeDescriptions_unknownClaimIDErrors(t *testing.T) {
	scored := `{
  "schema_version": 1,
  "role": "vision",
  "project": "p",
  "generated_at": "2026-01-01T00:00:00Z",
  "claims": [
    {"id": "claim-99", "status": "done", "evidence": "e1"}
  ]
}`
	_, err := MergeDescriptions([]byte(testClaimsJSON), []byte(scored))
	if err == nil {
		t.Fatal("expected an error for a claim id absent from the original claims list, got nil")
	}
}

func TestMergeDescriptions_droppedClaimErrors(t *testing.T) {
	// testClaimsJSON has two claims; this report only scores one — the
	// exact correlated-omission shape both real runs against this project
	// hit independently (README.md's Status section).
	scored := `{
  "schema_version": 1,
  "role": "vision",
  "project": "p",
  "generated_at": "2026-01-01T00:00:00Z",
  "claims": [
    {"id": "claim-01", "status": "done", "evidence": "e1"}
  ]
}`
	_, err := MergeDescriptions([]byte(testClaimsJSON), []byte(scored))
	if err == nil {
		t.Fatal("expected an error when the agent scored a subset of the original claims list, got nil")
	}
	if !strings.Contains(err.Error(), "claim-02") {
		t.Fatalf("expected the error to name the dropped claim claim-02, got: %v", err)
	}
}
