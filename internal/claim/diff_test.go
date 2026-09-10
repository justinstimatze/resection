package claim

import (
	"strings"
	"testing"
	"time"
)

func reportWith(role string, claims ...ScoredClaim) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Role:          role,
		Project:       "test-project",
		GeneratedAt:   time.Now().UTC(),
		Claims:        claims,
	}
}

func TestDiff_matching(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"})

	result, err := Diff(vision, impl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FlaggedCount != 0 {
		t.Fatalf("expected 0 flagged, got %d", result.FlaggedCount)
	}
	if !result.Rows[0].Match {
		t.Fatalf("expected row to match")
	}
}

func TestDiff_mismatchedStatus(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusNotStarted, Evidence: "i"})

	result, err := Diff(vision, impl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FlaggedCount != 1 {
		t.Fatalf("expected 1 flagged, got %d", result.FlaggedCount)
	}
	if result.Rows[0].Match || result.Rows[0].Flag != FlagStatusMismatch {
		t.Fatalf("expected a status_mismatch flag, got %+v", result.Rows[0])
	}
}

func TestDiff_evidenceDifferenceAloneNeverFlags(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "vision quote"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "path:42, totally different text"})

	result, err := Diff(vision, impl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FlaggedCount != 0 {
		t.Fatalf("evidence-only difference must not flag, got %d flagged", result.FlaggedCount)
	}
}

func TestDiff_missingInImpl(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"},
		ScoredClaim{ID: "claim-02", Description: "y", Status: StatusDone, Evidence: "v"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"})

	_, err := Diff(vision, impl)
	if err == nil || !strings.Contains(err.Error(), "missing in impl") {
		t.Fatalf("expected a missing-in-impl pipeline error, got: %v", err)
	}
}

func TestDiff_selfDiffRefused(t *testing.T) {
	// The exact false-clean shape a caller must never be able to produce
	// by accident: the same report handed to both --vision and --impl.
	sameSide := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"})

	_, err := Diff(sameSide, sameSide)
	if err == nil {
		t.Fatal("expected an error when both sides carry role \"implementation\", got nil")
	}
}

func TestDiff_wrongRoleErrors(t *testing.T) {
	swapped := reportWith(RoleImplementation, // wrong: this is passed as vision
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"})

	_, err := Diff(swapped, impl)
	if err == nil || !strings.Contains(err.Error(), "not a vision-side report") {
		t.Fatalf("expected a wrong-role error naming the vision side, got: %v", err)
	}
}

func TestDiff_projectMismatchErrors(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"})
	vision.Project = "project-a"
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"})
	impl.Project = "project-b"

	_, err := Diff(vision, impl)
	if err == nil || !strings.Contains(err.Error(), "different projects") {
		t.Fatalf("expected a project-mismatch error, got: %v", err)
	}
}

func TestDiff_missingInVision(t *testing.T) {
	vision := reportWith(RoleVision,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "v"})
	impl := reportWith(RoleImplementation,
		ScoredClaim{ID: "claim-01", Description: "x", Status: StatusDone, Evidence: "i"},
		ScoredClaim{ID: "claim-02", Description: "y", Status: StatusDone, Evidence: "i"})

	_, err := Diff(vision, impl)
	if err == nil || !strings.Contains(err.Error(), "missing in vision") {
		t.Fatalf("expected a missing-in-vision pipeline error, got: %v", err)
	}
}
