// Package store owns resection's on-disk layout under ~/.claude/resection:
// per-project directories keyed by an md5-prefix hash, and the atomic
// write helper every mutable file (counter.json, run artifacts) goes
// through. Path-helper shape mirrors hindcast's internal/store/store.go —
// same tool family, same person reads both later.
package store

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResectionDir is the root of all persisted resection state.
func ResectionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claude", "resection")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// ProjectsDir is where per-project state (counter.json, runs/) lives.
func ProjectsDir() (string, error) {
	root, err := ResectionDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "projects")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// ProjectHash returns the first 8 hex chars of md5(project) — a short,
// filesystem-safe directory name for an arbitrary project identifier.
func ProjectHash(project string) string {
	sum := md5.Sum([]byte(project))
	return hex.EncodeToString(sum[:])[:8]
}

// ResolveProject identifies "which project" a hook invocation belongs to,
// given the cwd Claude Code reports in its hook JSON. A `.resection-project`
// override file in cwd wins (for a project checked out under an unstable or
// symlinked path); otherwise the directory's base name.
func ResolveProject(cwd string) string {
	if data, err := os.ReadFile(filepath.Join(cwd, ".resection-project")); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			return name
		}
	}
	return filepath.Base(cwd)
}

// ResolveProjectArg resolves a CLI `--project` flag value the same way a
// PostCompact/SessionStart hook resolves its own cwd: empty defaults to
// the current working directory, and any non-empty value is treated as a
// project root path — never a pre-resolved identifier — and passed
// through ResolveProject. Every CLI entry point that takes `--project`
// (rundir, show, status) goes through this so a path-shaped argument
// hashes to the same project bucket a real hook invocation for that
// project would use, rather than hashing the path string itself.
func ResolveProjectArg(pathFlag string) (string, error) {
	path := pathFlag
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = cwd
	}
	return ResolveProject(path), nil
}

// ProjectDir returns (creating if needed) the per-project state directory
// for the given project identifier.
func ProjectDir(project string) (string, error) {
	dir, err := ProjectsDir()
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, ProjectHash(project))
	if err := os.MkdirAll(full, 0700); err != nil {
		return "", err
	}
	return full, nil
}

// RunDir returns (creating if needed) a fresh run directory for the given
// project, named by a UTC timestamp with a decimal numeric suffix
// ("-2", "-3", ...) if two runs land in the same second. Collision
// detection is via os.Mkdir, not a Stat-then-create check — MkdirAll
// succeeds silently on an already-existing directory, which would defeat
// the whole point of the check, and a Stat followed by a separate create
// call leaves a window where two concurrent callers can both observe
// "not there yet" and both proceed to use the same directory.
func RunDir(project string) (string, error) {
	pdir, err := ProjectDir(project)
	if err != nil {
		return "", err
	}
	runsDir := filepath.Join(pdir, "runs")
	if err := os.MkdirAll(runsDir, 0700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	dir := filepath.Join(runsDir, stamp)
	for i := 2; ; i++ {
		err := os.Mkdir(dir, 0700)
		if err == nil {
			return dir, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
		dir = filepath.Join(runsDir, fmt.Sprintf("%s-%d", stamp, i))
	}
}

// AtomicWriteJSON writes data to path via a same-directory temp file,
// chmod 0600, then rename — so a reader never observes a torn/partial
// file, and a crash mid-write leaves the old contents (or nothing)
// rather than a corrupt one. Ported from hindcast's GetSalt/Sketch.Save
// pattern, without the accompanying lock file. This makes one write
// atomic; it does not make a caller's read-modify-write sequence atomic
// against a concurrent writer — see counter.go's IncrementCompact and
// CheckAndReset, both of which do exactly that and accept the tradeoff
// deliberately.
func AtomicWriteJSON(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	if err := tmp.Chmod(0600); err != nil {
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
