package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome redirects os.UserHomeDir for the duration of a test —
// LogPath (and everything built on it) derives from it.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestGuard_panicIsCaughtAndLogged(t *testing.T) {
	withTempHome(t)

	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		Guard("test-hook", func() {
			panic("boom")
		})
	}()
	if didPanic {
		t.Fatal("Guard must not let a panic escape to the caller")
	}

	path, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading hook.log: %v", err)
	}
	if !strings.Contains(string(data), "PANIC boom") {
		t.Fatalf("expected hook.log to record the panic, got: %s", data)
	}
	if !strings.Contains(string(data), "[test-hook]") {
		t.Fatalf("expected hook.log to name the hook that panicked, got: %s", data)
	}
}

func TestGuard_normalReturnLogsNothing(t *testing.T) {
	withTempHome(t)
	Guard("test-hook", func() {})

	path, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no hook.log to be created for a hook that never logs, stat err: %v", err)
	}
}

func TestDecode_validAndInvalidJSON(t *testing.T) {
	withTempHome(t)

	type payload struct {
		CWD string `json:"cwd"`
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(`{"cwd": "/tmp/project"}`); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var p payload
	if err := Decode("test-hook", &p); err != nil {
		t.Fatalf("Decode on valid JSON: %v", err)
	}
	if p.CWD != "/tmp/project" {
		t.Fatalf("Decode: cwd = %q, want /tmp/project", p.CWD)
	}
}

func TestDecode_invalidJSONLogsAndErrors(t *testing.T) {
	withTempHome(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(`not json{{{`); err != nil {
		t.Fatal(err)
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var p struct{}
	if err := Decode("test-hook", &p); err == nil {
		t.Fatal("expected Decode to error on invalid JSON")
	}

	path, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading hook.log: %v", err)
	}
	if !strings.Contains(string(data), "stdin decode error") {
		t.Fatalf("expected hook.log to record the decode error, got: %s", data)
	}
}

func TestLogPath_createsDirAndReturnsStablePath(t *testing.T) {
	home := withTempHome(t)
	path, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	want := filepath.Join(home, ".claude", "resection", "hook.log")
	if path != want {
		t.Fatalf("LogPath() = %q, want %q", path, want)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
		t.Fatalf("expected LogPath to create its parent directory: %v", err)
	}
}

func TestLogf_rotatesPastMaxSize(t *testing.T) {
	withTempHome(t)

	orig := maxLogSize
	maxLogSize = 100 // shrink so a handful of lines trips rotation
	defer func() { maxLogSize = orig }()

	for i := 0; i < 20; i++ {
		Logf("test-hook", "line number %d of filler text to exceed the shrunk cap", i)
	}

	path, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected hook.log.1 to exist after exceeding maxLogSize, stat err: %v", err)
	}
}
