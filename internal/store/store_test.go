package store

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestResolveProject_basenameFallback(t *testing.T) {
	dir := t.TempDir()
	got := ResolveProject(dir)
	want := filepath.Base(dir)
	if got != want {
		t.Fatalf("ResolveProject(%q) = %q, want %q", dir, got, want)
	}
}

func TestResolveProject_overrideFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".resection-project"), []byte("my-project\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := ResolveProject(dir)
	if got != "my-project" {
		t.Fatalf("ResolveProject with override file = %q, want %q", got, "my-project")
	}
}

// TestResolveProjectArg_pathAndCwdAgree is the regression test for the
// bug this function fixes: a CLI caller passing a project's full root
// path via --project used to hash that path string directly (via
// store.RunDir(*project) with no resolution step), landing in a
// different project bucket than a PostCompact/SessionStart hook's own
// store.ResolveProject(cwd) resolution of the exact same directory ever
// would. ResolveProjectArg must make the two agree.
func TestResolveProjectArg_pathAndCwdAgree(t *testing.T) {
	dir := t.TempDir()

	viaPath, err := ResolveProjectArg(dir)
	if err != nil {
		t.Fatalf("ResolveProjectArg(path): %v", err)
	}

	viaHookCWD := ResolveProject(dir)

	if viaPath != viaHookCWD {
		t.Fatalf("ResolveProjectArg(%q) = %q, but a hook's own ResolveProject(cwd) for the same directory = %q — these must agree, or a CLI-driven run and an automatic hook fire for the same project land in different state buckets", dir, viaPath, viaHookCWD)
	}
}

// safeRunDirName matches RunDir's own naming scheme: a timestamp, optionally
// followed by a decimal collision suffix — never the raw-rune arithmetic
// ("-9", then "-:", "-;", "-<" for i=10..12) the old implementation produced,
// which put shell- and Windows-hostile characters into a directory name.
var safeRunDirName = regexp.MustCompile(`^[0-9TZ]+(-[0-9]+)?$`)

func TestRunDir_manyCallsProduceUniqueSafeNames(t *testing.T) {
	withTempHome(t)
	project := "test-project"

	const n = 15
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		dir, err := RunDir(project)
		if err != nil {
			t.Fatalf("RunDir call %d: %v", i, err)
		}
		base := filepath.Base(dir)
		if !safeRunDirName.MatchString(base) {
			t.Fatalf("RunDir call %d produced an unsafe directory name %q", i, base)
		}
		if seen[dir] {
			t.Fatalf("RunDir call %d returned a directory already returned by an earlier call: %s", i, dir)
		}
		seen[dir] = true
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("RunDir call %d: %s was not actually created as a directory: %v", i, dir, err)
		}
	}
}

func TestResolveProjectArg_emptyDefaultsToCWD(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	got, err := ResolveProjectArg("")
	if err != nil {
		t.Fatalf("ResolveProjectArg(\"\"): %v", err)
	}
	want := filepath.Base(dir)
	if got != want {
		t.Fatalf("ResolveProjectArg(\"\") = %q, want %q (basename of cwd)", got, want)
	}
}
