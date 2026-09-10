package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Counter is the per-project Nth-compact trigger state (docs/design.md
// "Trigger condition"). CompactsSinceFire resets to 0 every
// time CheckAndReset fires; TotalCompacts and FireCount never reset —
// they're the lifetime tallies used to pick a real N empirically once v1
// has run a while.
type Counter struct {
	SchemaVersion     int       `json:"schema_version"`
	Project           string    `json:"project"`
	CompactsSinceFire int       `json:"compacts_since_fire"`
	TotalCompacts     int       `json:"total_compacts"`
	FireCount         int       `json:"fire_count"`
	LastFiredAt       time.Time `json:"last_fired_at,omitempty"`
}

const counterSchemaVersion = 1

func counterPath(project string) (string, error) {
	dir, err := ProjectDir(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "counter.json"), nil
}

// loadOrDefault treats a missing or corrupt counter.json as "start fresh
// at zero" rather than propagating the error — a torn or corrupted file
// self-heals on the next hook call instead of wedging every future run.
func loadOrDefault(path, project string) Counter {
	data, err := os.ReadFile(path)
	if err != nil {
		return Counter{SchemaVersion: counterSchemaVersion, Project: project}
	}
	var c Counter
	if err := json.Unmarshal(data, &c); err != nil {
		return Counter{SchemaVersion: counterSchemaVersion, Project: project}
	}
	c.Project = project
	c.SchemaVersion = counterSchemaVersion
	return c
}

func writeCounter(path string, c Counter) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteJSON(path, data)
}

// IncrementCompact and CheckAndReset both read-modify-write counter.json:
// load, mutate a Go value, write it back via AtomicWriteJSON. That write
// is atomic; the sequence around it isn't — two hook invocations for the
// same project racing within the same window can still lose one
// increment or one fire. Accepted deliberately rather than adding a lock
// file: the counter is a soft, self-healing heuristic (loadOrDefault
// resets a corrupt file to zero) feeding a trigger whose N is itself
// picked empirically, not a count anything downstream depends on being
// exact.

// IncrementCompact bumps the counter after one PostCompact event. Pure
// side effect — never returns whether the Nth threshold was crossed;
// that check belongs to CheckAndReset, called separately from the
// SessionStart hook, since the two events can't share in-process state.
func IncrementCompact(project string) error {
	path, err := counterPath(project)
	if err != nil {
		return err
	}
	c := loadOrDefault(path, project)
	c.CompactsSinceFire++
	c.TotalCompacts++
	return writeCounter(path, c)
}

// CheckAndReset reports whether the project has reached n compacts since
// the last fire. If so, it resets CompactsSinceFire to 0 and bumps
// FireCount/LastFiredAt as part of the same write — the caller (the
// SessionStart hook) should treat a true return as "fired exactly once,
// state already updated," not something to re-check.
func CheckAndReset(project string, n int) (fired bool, c Counter, err error) {
	path, err := counterPath(project)
	if err != nil {
		return false, Counter{}, err
	}
	c = loadOrDefault(path, project)
	if n <= 0 || c.CompactsSinceFire < n {
		return false, c, nil
	}
	c.CompactsSinceFire = 0
	c.FireCount++
	c.LastFiredAt = time.Now().UTC()
	if err := writeCounter(path, c); err != nil {
		return false, c, err
	}
	return true, c, nil
}
