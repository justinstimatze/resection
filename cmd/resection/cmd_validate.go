package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/resection/internal/claim"
)

// cmdValidate implements `resection validate --role R --in FILE`, exiting
// non-zero with a specific, addressable error on failure — worded so it
// can be handed straight back to the generating agent as a repair
// instruction (see skills/resection/SKILL.md step 7).
func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	role := fs.String("role", "", "vision | implementation")
	in := fs.String("in", "", "path to the Report JSON file")
	_ = fs.Parse(args)

	if *role == "" || *in == "" {
		fmt.Fprintln(os.Stderr, "usage: resection validate --role vision|implementation --in FILE")
		os.Exit(2)
	}
	if *role != claim.RoleVision && *role != claim.RoleImplementation {
		fmt.Fprintf(os.Stderr, "resection validate: role must be %q or %q, got %q\n",
			claim.RoleVision, claim.RoleImplementation, *role)
		os.Exit(2)
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection validate: %s\n", err)
		os.Exit(1)
	}
	var r claim.Report
	if err := json.Unmarshal(data, &r); err != nil {
		fmt.Fprintf(os.Stderr, "resection validate: invalid JSON: %s\n", err)
		os.Exit(1)
	}
	if err := claim.Validate(r, *role); err != nil {
		fmt.Fprintf(os.Stderr, "resection validate: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok: %d claims, role=%s\n", len(r.Claims), r.Role)
}
