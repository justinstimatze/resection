// Package hook is the shared runtime for resection's Claude Code hooks:
// panic-guard, stdin-JSON decode, and hook.log logging. Every hook
// subcommand entry point wraps its body with Guard so a panic can never
// reach Claude. Ported from hindcast's internal/hook/hook.go — same
// tool shape, same person reads both later.
package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			Logf(name, "PANIC %v", r)
		}
	}()
	fn()
}

func Decode(name string, v any) error {
	if err := json.NewDecoder(os.Stdin).Decode(v); err != nil {
		Logf(name, "stdin decode error: %s", err)
		return err
	}
	return nil
}

func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claude", "resection")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "hook.log"), nil
}

// maxLogSize triggers rotation: when hook.log passes this, it moves to
// hook.log.1 (the previous .1 is discarded). Bounded at 10 MB so the log
// can't fill disk under long-running use. A var, not a const, so tests
// can shrink it to exercise rotation without writing 10 MB of log lines.
var maxLogSize int64 = 10 * 1024 * 1024

func Logf(name, format string, args ...any) {
	path, err := LogPath()
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s] %s\n",
		time.Now().UTC().Format(time.RFC3339),
		name,
		fmt.Sprintf(format, args...),
	)
	if stat, err := f.Stat(); err == nil && stat.Size() > maxLogSize {
		// Best-effort rotation: ignore errors to keep the logger
		// guaranteed non-blocking.
		_ = os.Rename(path, path+".1")
	}
}
