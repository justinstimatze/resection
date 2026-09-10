package store

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempHome redirects os.UserHomeDir's effect for the duration of a
// test by pointing $HOME at a temp dir — ResectionDir and everything
// built on it derive from os.UserHomeDir(), so this isolates each test's
// state without touching the real ~/.claude/resection/.
func withTempHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
}

func TestCounter_incrementThenFireAndReset(t *testing.T) {
	withTempHome(t)
	project := "test-project"

	for i := 0; i < 3; i++ {
		if err := IncrementCompact(project); err != nil {
			t.Fatalf("IncrementCompact: %v", err)
		}
	}

	fired, c, err := CheckAndReset(project, 5)
	if err != nil {
		t.Fatalf("CheckAndReset: %v", err)
	}
	if fired {
		t.Fatalf("expected not fired at 3/5, got fired with %+v", c)
	}
	if c.CompactsSinceFire != 3 {
		t.Fatalf("expected compacts_since_fire=3, got %d", c.CompactsSinceFire)
	}

	for i := 0; i < 2; i++ {
		if err := IncrementCompact(project); err != nil {
			t.Fatalf("IncrementCompact: %v", err)
		}
	}

	fired, c, err = CheckAndReset(project, 5)
	if err != nil {
		t.Fatalf("CheckAndReset: %v", err)
	}
	if !fired {
		t.Fatalf("expected fired at 5/5, got %+v", c)
	}
	if c.CompactsSinceFire != 0 {
		t.Fatalf("expected compacts_since_fire reset to 0, got %d", c.CompactsSinceFire)
	}
	if c.FireCount != 1 {
		t.Fatalf("expected fire_count=1, got %d", c.FireCount)
	}
	if c.TotalCompacts != 5 {
		t.Fatalf("expected total_compacts=5, got %d", c.TotalCompacts)
	}

	// A second check right after firing, with nothing incremented since,
	// must not fire again.
	fired, _, err = CheckAndReset(project, 5)
	if err != nil {
		t.Fatalf("CheckAndReset: %v", err)
	}
	if fired {
		t.Fatalf("expected not fired immediately after a reset")
	}
}

func TestCounter_corruptFileSelfHeals(t *testing.T) {
	withTempHome(t)
	project := "test-project"

	if err := IncrementCompact(project); err != nil {
		t.Fatalf("IncrementCompact: %v", err)
	}
	path, err := counterPath(project)
	if err != nil {
		t.Fatalf("counterPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("not json{{{"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Next increment must not error out on the corrupt file — it should
	// start fresh at zero (plus this one increment) rather than propagate
	// the parse error.
	if err := IncrementCompact(project); err != nil {
		t.Fatalf("IncrementCompact after corruption: %v", err)
	}
	_, c, err := CheckAndReset(project, 1)
	if err != nil {
		t.Fatalf("CheckAndReset: %v", err)
	}
	if c.CompactsSinceFire != 0 {
		t.Fatalf("expected the corrupt-file recovery to have reset and re-incremented to 1 then fired, got %+v", c)
	}
}

func TestCounter_zeroOrNegativeNNeverFires(t *testing.T) {
	withTempHome(t)
	project := "test-project"
	if err := IncrementCompact(project); err != nil {
		t.Fatalf("IncrementCompact: %v", err)
	}
	fired, _, err := CheckAndReset(project, 0)
	if err != nil {
		t.Fatalf("CheckAndReset: %v", err)
	}
	if fired {
		t.Fatalf("N<=0 must never fire")
	}
}
