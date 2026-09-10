package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withTempHome redirects os.UserHomeDir for the duration of a test — every
// function under test here (mergeClaudeSettings, unmergeClaudeSettings,
// claudeAgentsDir, claudeSkillsDir) derives its path from it.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestIsResectionCmd(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"/home/user/go/bin/resection compact-count", true},
		{"resection compact-check", true},
		{"/some/path/resection-bin compact-count", true}, // renamed/scratch binary
		{"RESECTION compact-check", true},                // case-insensitive basename match
		{"resection", false},                             // no subcommand at all
		{"resection foo", false},                         // unrelated subcommand
		{"otherbinary compact-count", false},             // "resection" not in basename
		{"", false},
		{"resectionista compact-count", true}, // substring match is intentionally loose
	}
	for _, c := range cases {
		if got := isResectionCmd(c.cmd); got != c.want {
			t.Errorf("isResectionCmd(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestAddHook_dedupesByCommand(t *testing.T) {
	hooks := map[string]any{}
	addHook(hooks, "PostCompact", "resection compact-count")
	addHook(hooks, "PostCompact", "resection compact-count") // duplicate, must not append
	addHook(hooks, "PostCompact", "resection compact-check") // different command, must append

	list, ok := hooks["PostCompact"].([]any)
	if !ok {
		t.Fatalf("hooks[PostCompact] is not a list: %#v", hooks["PostCompact"])
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 distinct hook entries after a duplicate add, got %d: %#v", len(list), list)
	}
}

func TestLooksLikeRepoRoot(t *testing.T) {
	dir := t.TempDir()
	if looksLikeRepoRoot(dir) {
		t.Fatal("empty directory should not look like the repo root")
	}
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0700); err != nil {
		t.Fatal(err)
	}
	if looksLikeRepoRoot(dir) {
		t.Fatal("agents/ alone (no skills/resection/) should not look like the repo root")
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "resection"), 0700); err != nil {
		t.Fatal(err)
	}
	if !looksLikeRepoRoot(dir) {
		t.Fatal("agents/ + skills/resection/ should look like the repo root")
	}
}

func TestSymlinkAsset(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "thing.md")
	if err := os.WriteFile(src, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	dstDir := t.TempDir()

	if err := symlinkAsset(src, dstDir, "thing.md"); err != nil {
		t.Fatalf("first symlinkAsset: %v", err)
	}
	if err := checkSymlinkAsset(src, dstDir, "thing.md"); err != nil {
		t.Fatalf("checkSymlinkAsset after a correct link should pass: %v", err)
	}
	// Idempotent: calling again with the same src is a no-op, not an error.
	if err := symlinkAsset(src, dstDir, "thing.md"); err != nil {
		t.Fatalf("second symlinkAsset (idempotent case): %v", err)
	}

	// Refuses to clobber a real file.
	realDst := t.TempDir()
	if err := os.WriteFile(filepath.Join(realDst, "thing.md"), []byte("real file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkSymlinkAsset(src, realDst, "thing.md"); err == nil {
		t.Fatal("expected checkSymlinkAsset to refuse a destination that's a real file")
	}
	if err := symlinkAsset(src, realDst, "thing.md"); err == nil {
		t.Fatal("expected symlinkAsset to refuse a destination that's a real file")
	}

	// Refuses to clobber a symlink pointing elsewhere.
	otherSrc := filepath.Join(srcDir, "other.md")
	if err := os.WriteFile(otherSrc, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	wrongDst := t.TempDir()
	if err := os.Symlink(otherSrc, filepath.Join(wrongDst, "thing.md")); err != nil {
		t.Fatal(err)
	}
	if err := checkSymlinkAsset(src, wrongDst, "thing.md"); err == nil {
		t.Fatal("expected checkSymlinkAsset to refuse a destination symlinked elsewhere")
	}
	if err := symlinkAsset(src, wrongDst, "thing.md"); err == nil {
		t.Fatal("expected symlinkAsset to refuse a destination symlinked elsewhere")
	}
}

func TestMergeAndUnmergeClaudeSettings_roundTrip(t *testing.T) {
	home := withTempHome(t)
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"model": "opus"}`)
	if err := os.WriteFile(settingsPath, original, 0600); err != nil {
		t.Fatal(err)
	}

	if err := mergeClaudeSettings("/fake/path/resection"); err != nil {
		t.Fatalf("mergeClaudeSettings: %v", err)
	}

	merged := readSettings(t, settingsPath)
	hooks, ok := merged["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected a hooks key after merge, got: %#v", merged)
	}
	if _, ok := hooks["PostCompact"]; !ok {
		t.Fatal("expected PostCompact hook after merge")
	}
	if _, ok := hooks["SessionStart"]; !ok {
		t.Fatal("expected SessionStart hook after merge")
	}
	if merged["model"] != "opus" {
		t.Fatalf("merge must preserve unrelated existing keys, got model=%v", merged["model"])
	}
	if len(findBackups(t, settingsPath)) != 1 {
		t.Fatalf("expected exactly 1 backup after merge, got %d", len(findBackups(t, settingsPath)))
	}

	removed, err := unmergeClaudeSettings()
	if err != nil {
		t.Fatalf("unmergeClaudeSettings: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 hook entries removed, got %d", removed)
	}
	final := readSettings(t, settingsPath)
	if _, ok := final["hooks"]; ok {
		t.Fatalf("expected hooks key gone entirely once both entries are removed, got: %#v", final["hooks"])
	}
	if final["model"] != "opus" {
		t.Fatalf("unmerge must preserve unrelated existing keys, got model=%v", final["model"])
	}
	if len(findBackups(t, settingsPath)) != 2 {
		t.Fatalf("expected a second backup after unmerge, got %d total", len(findBackups(t, settingsPath)))
	}
}

func TestUnmergeClaudeSettings_noopLeavesFileUntouched(t *testing.T) {
	home := withTempHome(t)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatal(err)
	}
	// A hooks section that exists but contains nothing resection would recognize.
	original := []byte(`{"hooks": {"PostCompact": [{"matcher": "", "hooks": [{"type": "command", "command": "some-other-tool run"}]}]}}`)
	if err := os.WriteFile(settingsPath, original, 0600); err != nil {
		t.Fatal(err)
	}

	removed, err := unmergeClaudeSettings()
	if err != nil {
		t.Fatalf("unmergeClaudeSettings: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed when no resection hooks are present, got %d", removed)
	}
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("a no-op unmerge must not rewrite the file:\nbefore: %s\nafter:  %s", original, after)
	}
	if backups := findBackups(t, settingsPath); len(backups) != 0 {
		t.Fatalf("a no-op unmerge must not take a backup, found %d", len(backups))
	}
}

func TestUnmergeClaudeSettings_missingFileIsNoop(t *testing.T) {
	withTempHome(t)
	removed, err := unmergeClaudeSettings()
	if err != nil {
		t.Fatalf("unmergeClaudeSettings on a missing settings.json: %v", err)
	}
	if removed != 0 {
		t.Fatalf("expected 0 removed for a missing file, got %d", removed)
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("%s is not valid JSON: %v", path, err)
	}
	return settings
}

func findBackups(t *testing.T, settingsPath string) []string {
	t.Helper()
	matches, err := filepath.Glob(settingsPath + ".resection-backup-*")
	if err != nil {
		t.Fatalf("globbing for backups: %v", err)
	}
	return matches
}
