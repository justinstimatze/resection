// Command resection is the entry point for all resection subcommands: the
// Claude Code hooks (compact-count, compact-check), the schema/diff CLI
// used by skills/resection/SKILL.md, and the user-facing install/uninstall.
//
// All subcommands share a single Go binary so settings.json only has to
// point at one executable path.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/justinstimatze/resection/internal/hook"
)

// version is "dev" by default and baked at release time via
//
//	go install -ldflags "-X main.version=$(git describe --tags --always --dirty)" ./cmd/resection
//
// The git tag is the single source of truth — there is no hand-maintained
// version constant to drift out of sync. buildVersion() resolves it.
var version = "dev"

// buildVersion reports the binary's version, preferring (in order): a
// release value baked in via -ldflags; the module version when installed
// with `go install …@vX.Y.Z`; the embedded VCS commit (+dirty) for local
// `go build`. Falls back to "dev" when none is available.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				rev = s.Value[:12]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" {
		return rev + dirty
	}
	return version
}

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}
	args := os.Args[1:]
	switch args[0] {
	// Hooks — invoked by Claude Code via settings.json; never run by hand.
	case "compact-count":
		hook.Guard("compact-count", cmdCompactCount)
	case "compact-check":
		hook.Guard("compact-check", cmdCompactCheck)

	// CLI used by skills/resection/SKILL.md's orchestration.
	case "validate":
		cmdValidate(args[1:])
	case "diff":
		cmdDiff(args[1:])
	case "assign-ids":
		cmdAssignIDs(args[1:])
	case "merge-descriptions":
		cmdMergeDescriptions(args[1:])
	case "rundir":
		cmdRunDir(args[1:])

	// User-facing CLI.
	case "install":
		cmdInstall(args[1:])
	case "uninstall":
		cmdUninstall(args[1:])
	case "show":
		cmdShow(args[1:])
	case "status":
		cmdStatus(args[1:])

	case "version", "--version", "-v":
		fmt.Println("resection", buildVersion())
	case "help", "--help", "-h":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "resection: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `resection — vision/implementation drift detection for a project

Usage:
  resection install               Symlink the two agent types and the skill
                                  into ~/.claude/. Pass --with-hooks to also
                                  wire the Nth-compact trigger — leave off
                                  until a real N is chosen empirically.
  resection uninstall             Remove hook entries and symlinks
                                  (--keep-data to preserve run history).
  resection show [--project PATH]     List past runs for a project.
  resection status [--project PATH]   Hook health check (tail of hook.log).
  resection version               Print version.

CLI used by the resection skill's orchestration (see skills/resection/SKILL.md):
  resection assign-ids            Number a bare claim-stub JSON array.
  resection merge-descriptions --claims F --in F --out F   Re-attach each
                                  claim's description from the claims list
                                  into a scored Report, keyed by id — an
                                  agent's score-mode dispatch never has to
                                  reproduce it correctly.
  resection validate --role R --in FILE   Validate a Report against the schema.
  resection diff --vision F --impl F --out F   Deterministic field-by-field diff.
  resection rundir [--project PATH]   Print (creating) a fresh run directory
                                  path. PATH is a project root, default cwd —
                                  resolved the same way the hooks resolve
                                  their own cwd, not a pre-resolved identifier.

Hooks (invoked by Claude Code; not for manual use):
  resection compact-count         PostCompact — increment the Nth-compact counter.
  resection compact-check         SessionStart — fire the trigger on every Nth compact.

See docs/design.md for why it's built this way, README.md for what it does.
`)
}
