package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justinstimatze/resection/internal/store"
)

// agentFiles and skillName are the two on-disk asset kinds resection
// ships alongside the compiled binary — symlinked into ~/.claude/, same
// convention already on disk for costrel-consultant.md.
var agentFiles = []string{
	"resection-vision-reader.md",
	"resection-implementation-reader.md",
}

const skillName = "resection"

func cmdInstall(args []string) {
	fl := flag.NewFlagSet("install", flag.ExitOnError)
	withHooks := fl.Bool("with-hooks", false, "also wire the PostCompact/SessionStart "+
		"Nth-compact trigger into settings.json — leave off until a real N has been "+
		"picked empirically; manual invocation works fully without it")
	_ = fl.Parse(args)

	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection install: %s\n", err)
		os.Exit(1)
	}
	if !looksLikeRepoRoot(repoRoot) {
		fmt.Fprintln(os.Stderr, "resection install: run from the resection repo root "+
			"(agents/ and skills/resection/ not found here)")
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection install: cannot find own path: %s\n", err)
		os.Exit(1)
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}

	fmt.Fprintf(os.Stderr, "resection install (binary: %s, repo: %s)\n", exe, repoRoot)

	// Validate every symlink precondition BEFORE touching settings.json —
	// settings.json is global state shared with every other Claude Code
	// project, so a conflict discovered here must never leave it merged
	// while the agent/skill symlinks it depends on are only half in
	// place. This is a dry run: it checks, it does not create anything.
	for _, name := range agentFiles {
		if err := checkSymlinkAsset(filepath.Join(repoRoot, "agents", name), claudeAgentsDir(), name); err != nil {
			fmt.Fprintf(os.Stderr, "  agent %s: %s\n", name, err)
			os.Exit(1)
		}
	}
	if err := checkSymlinkAsset(filepath.Join(repoRoot, "skills", skillName), claudeSkillsDir(), skillName); err != nil {
		fmt.Fprintf(os.Stderr, "  skill %s: %s\n", skillName, err)
		os.Exit(1)
	}

	if *withHooks {
		if err := mergeClaudeSettings(exe); err != nil {
			fmt.Fprintf(os.Stderr, "  settings.json: %s\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "  settings.json: PostCompact + SessionStart hooks merged")
	} else {
		fmt.Fprintln(os.Stderr, "  settings.json: skipped (pass --with-hooks to wire the "+
			"Nth-compact trigger once a real N has been chosen; manual invocation works without it)")
	}

	for _, name := range agentFiles {
		if err := symlinkAsset(filepath.Join(repoRoot, "agents", name), claudeAgentsDir(), name); err != nil {
			// The precondition check above already validated this — reaching
			// here means the filesystem changed underneath us since then.
			// settings.json may already be written; say so rather than
			// leaving that implicit.
			fmt.Fprintf(os.Stderr, "  agent %s: %s (settings.json changes above, if any, are already applied)\n", name, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "  agent: %s symlinked\n", name)
	}
	if err := symlinkAsset(filepath.Join(repoRoot, "skills", skillName), claudeSkillsDir(), skillName); err != nil {
		fmt.Fprintf(os.Stderr, "  skill %s: %s (settings.json changes above, if any, are already applied)\n", skillName, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "  skill: %s symlinked\n", skillName)

	fmt.Fprintln(os.Stderr, "\nresection installed. Manual invocation: run the resection skill against any project.")
	if *withHooks {
		fmt.Fprintln(os.Stderr, "The Nth-compact trigger fires automatically once N is reached.")
	}
}

func cmdUninstall(args []string) {
	fl := flag.NewFlagSet("uninstall", flag.ExitOnError)
	keepData := fl.Bool("keep-data", false, "keep counters + run history in ~/.claude/resection/")
	_ = fl.Parse(args)

	exitCode := 0

	switch removed, err := unmergeClaudeSettings(); {
	case err != nil:
		fmt.Fprintf(os.Stderr, "  settings.json: %s\n", err)
		exitCode = 1
	case removed == 0:
		fmt.Fprintln(os.Stderr, "  settings.json: no resection hooks found, nothing to remove")
	default:
		plural := "y"
		if removed != 1 {
			plural = "ies"
		}
		fmt.Fprintf(os.Stderr, "  settings.json: %d resection hook entr%s removed (backup taken)\n", removed, plural)
	}

	for _, name := range agentFiles {
		removeAssetIfOurs(filepath.Join(claudeAgentsDir(), name))
	}
	removeAssetIfOurs(filepath.Join(claudeSkillsDir(), skillName))

	if !*keepData {
		if root, err := store.ResectionDir(); err == nil {
			if err := os.RemoveAll(root); err == nil {
				fmt.Fprintf(os.Stderr, "  data: %s removed\n", root)
			} else {
				fmt.Fprintf(os.Stderr, "  data: remove failed: %s\n", err)
				exitCode = 1
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "  data: kept (omit --keep-data to delete)")
	}

	if exitCode == 0 {
		fmt.Fprintln(os.Stderr, "\nresection uninstalled.")
	} else {
		fmt.Fprintln(os.Stderr, "\nresection uninstall finished with errors — see above.")
	}
	os.Exit(exitCode)
}

func looksLikeRepoRoot(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "agents")); err != nil || !info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "skills", skillName)); err != nil || !info.IsDir() {
		return false
	}
	return true
}

func claudeAgentsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "agents")
}

func claudeSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills")
}

// checkSymlinkAsset reports whether symlinkAsset(src, dstDir, name) would
// succeed, without creating or modifying anything — so cmdInstall can
// validate every symlink precondition before it writes to settings.json,
// a piece of global state shared with every other project.
func checkSymlinkAsset(src, dstDir, name string) error {
	dst := filepath.Join(dstDir, name)
	if target, err := os.Readlink(dst); err == nil {
		if target == src {
			return nil // already correctly linked
		}
		return fmt.Errorf("%s already symlinks elsewhere (%s) — refusing to overwrite", dst, target)
	} else if _, statErr := os.Lstat(dst); statErr == nil {
		return fmt.Errorf("%s already exists and is not a symlink — refusing to overwrite", dst)
	}
	return nil
}

// symlinkAsset creates dstDir/name -> src, idempotent: a no-op if the
// existing entry is already a symlink pointing at src, and a refusal
// (not a clobber) if it's a real file or points somewhere else.
func symlinkAsset(src, dstDir, name string) error {
	if err := checkSymlinkAsset(src, dstDir, name); err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0700); err != nil {
		return err
	}
	dst := filepath.Join(dstDir, name)
	if target, err := os.Readlink(dst); err == nil && target == src {
		return nil // already correctly linked
	}
	return os.Symlink(src, dst)
}

// removeAssetIfOurs removes dst only if it's a symlink into this
// checkout's tree — never a real file, and never a symlink pointing
// somewhere else (someone else's install, or a user override).
func removeAssetIfOurs(dst string) {
	target, err := os.Readlink(dst)
	if err != nil {
		return // not a symlink (or doesn't exist) — leave it alone
	}
	if !strings.Contains(target, string(filepath.Separator)+"resection"+string(filepath.Separator)) &&
		!strings.HasSuffix(target, string(filepath.Separator)+"resection") {
		return
	}
	if err := os.Remove(dst); err == nil {
		fmt.Fprintf(os.Stderr, "  removed %s\n", dst)
	}
}

// backupPath picks a settings.json backup filename that doesn't already
// exist: the timestamp alone is only second-granularity, so an install
// immediately followed by an uninstall (or vice versa) can otherwise
// generate the same name twice and the second write silently clobbers
// the first backup — same collision shape as store.RunDir, same fix.
func backupPath(settingsPath string) string {
	base := settingsPath + ".resection-backup-" + time.Now().Format("20060102-150405")
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

func mergeClaudeSettings(exe string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	var settingsMode os.FileMode = 0600
	var settings map[string]any
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if info, statErr := os.Stat(path); statErr == nil {
			settingsMode = info.Mode().Perm()
		}
		if err := os.WriteFile(backupPath(path), data, 0600); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("existing settings.json is invalid JSON: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		settings = map[string]any{}
	default:
		return err
	}
	if settings == nil {
		settings = map[string]any{}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	addHook(hooks, "PostCompact", exe+" compact-count")
	addHook(hooks, "SessionStart", exe+" compact-check")
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), settingsMode)
}

// addHook appends a resection hook entry unless one with the same
// command is already registered.
func addHook(hooks map[string]any, event, cmd string) {
	existing, _ := hooks[event].([]any)
	for _, entry := range existing {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := em["hooks"].([]any)
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if c, _ := hm["command"].(string); c == cmd {
				return
			}
		}
	}
	existing = append(existing, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{"type": "command", "command": cmd},
		},
	})
	hooks[event] = existing
}

// unmergeClaudeSettings removes resection's own hook entries from
// settings.json and reports how many it removed, so cmdUninstall can
// print an accurate message instead of declaring success on a no-op. It
// leaves the file completely untouched (no rewrite, no backup) when there
// was nothing to remove, and — symmetric with mergeClaudeSettings — takes
// a timestamped backup before it writes anything that does change.
func unmergeClaudeSettings() (removed int, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}
	path := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var settingsMode os.FileMode = 0600
	if info, statErr := os.Stat(path); statErr == nil {
		settingsMode = info.Mode().Perm()
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return 0, fmt.Errorf("settings.json is invalid JSON: %w", err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	for event, val := range hooks {
		list, ok := val.([]any)
		if !ok {
			continue
		}
		filtered := list[:0]
	entry:
		for _, entry := range list {
			em, ok := entry.(map[string]any)
			if !ok {
				filtered = append(filtered, entry)
				continue
			}
			inner, _ := em["hooks"].([]any)
			for _, h := range inner {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				if c, _ := hm["command"].(string); isResectionCmd(c) {
					removed++
					continue entry
				}
			}
			filtered = append(filtered, entry)
		}
		if len(filtered) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = filtered
		}
	}
	if removed == 0 {
		return 0, nil
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	if err := os.WriteFile(backupPath(path), data, 0600); err != nil {
		return 0, fmt.Errorf("backup: %w", err)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, append(out, '\n'), settingsMode); err != nil {
		return 0, err
	}
	return removed, nil
}

// isResectionCmd returns true when a settings.json command string looks
// like one resection registered — some path whose binary name *contains*
// "resection" (covers a renamed or scratch build like "resection-bin",
// not just an exact match — an exact-match check here previously let
// uninstall silently fail to recognize and remove a hook installed from
// a differently-named binary) followed by "compact-count" or
// "compact-check". Those two subcommand names are resection's own
// private vocabulary, so requiring both the substring match and the
// exact subcommand keeps this from false-matching an unrelated tool.
func isResectionCmd(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	base := strings.ToLower(filepath.Base(fields[0]))
	return strings.Contains(base, "resection") &&
		(fields[1] == "compact-count" || fields[1] == "compact-check")
}
