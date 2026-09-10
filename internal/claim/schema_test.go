package claim

import (
	"strings"
	"testing"
	"time"
)

func validReport() Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Role:          RoleVision,
		Project:       "test-project",
		GeneratedAt:   time.Now().UTC(),
		Claims: []ScoredClaim{
			{ID: "claim-01", Description: "does a thing", Status: StatusDone, Evidence: "README.md says so"},
			{
				ID: "claim-02", Description: "does the important thing", Headline: true,
				Status: StatusDone, Evidence: "a function named for it exists",
				FalsificationCriterion: "would look identical if it were an easier substitute; not ruled out",
			},
		},
	}
}

func TestValidate_valid(t *testing.T) {
	if err := Validate(validReport(), RoleVision); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_wrongRole(t *testing.T) {
	err := Validate(validReport(), RoleImplementation)
	if err == nil || !strings.Contains(err.Error(), "role") {
		t.Fatalf("expected a role error, got: %v", err)
	}
}

func TestValidate_wrongSchemaVersion(t *testing.T) {
	r := validReport()
	r.SchemaVersion = 999
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected a schema_version error, got: %v", err)
	}
}

func TestValidate_missingID(t *testing.T) {
	r := validReport()
	r.Claims[0].ID = ""
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "missing id") {
		t.Fatalf("expected a missing-id error, got: %v", err)
	}
}

func TestValidate_duplicateID(t *testing.T) {
	r := validReport()
	r.Claims[1].ID = r.Claims[0].ID
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected a duplicate-id error, got: %v", err)
	}
}

func TestValidate_badStatus(t *testing.T) {
	r := validReport()
	r.Claims[0].Status = "in-progress"
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "not one of done/partial/not-started/contradicted") {
		t.Fatalf("expected a bad-status error, got: %v", err)
	}
}

func TestValidate_missingEvidence(t *testing.T) {
	r := validReport()
	r.Claims[0].Evidence = ""
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "missing evidence") {
		t.Fatalf("expected a missing-evidence error, got: %v", err)
	}
}

func TestValidate_headlineMissingFalsificationCriterion(t *testing.T) {
	r := validReport()
	r.Claims[1].FalsificationCriterion = ""
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "falsification_criterion") {
		t.Fatalf("expected a headline/falsification_criterion error, got: %v", err)
	}
}

func TestValidate_nonHeadlineDoesNotRequireFalsificationCriterion(t *testing.T) {
	r := validReport()
	// claim-01 is non-headline with no falsification_criterion — must pass.
	if err := Validate(r, RoleVision); err != nil {
		t.Fatalf("non-headline claim without falsification_criterion should be valid, got: %v", err)
	}
}

func TestValidate_emptyClaims(t *testing.T) {
	r := validReport()
	r.Claims = nil
	err := Validate(r, RoleVision)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected an empty-claims error, got: %v", err)
	}
}
