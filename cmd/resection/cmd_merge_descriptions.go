package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/resection/internal/claim"
)

// cmdMergeDescriptions implements `resection merge-descriptions`, run by
// the skill's orchestration between extracting a score-mode agent's raw
// JSON and validating it — see claim.MergeDescriptions for why this
// exists rather than relying on the agent to reproduce description.
func cmdMergeDescriptions(args []string) {
	fs := flag.NewFlagSet("merge-descriptions", flag.ExitOnError)
	claimsPath := fs.String("claims", "", "claims.json — the bare claim-stub list (id, description, headline)")
	inPath := fs.String("in", "", "raw scored Report JSON from an agent dispatch")
	outPath := fs.String("out", "", "output path for the Report JSON with description re-attached")
	_ = fs.Parse(args)

	if *claimsPath == "" || *inPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: resection merge-descriptions --claims claims.json --in raw.json --out out.json")
		os.Exit(2)
	}

	claimsData, err := os.ReadFile(*claimsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection merge-descriptions: %s\n", err)
		os.Exit(1)
	}
	scoredData, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection merge-descriptions: %s\n", err)
		os.Exit(1)
	}

	merged, err := claim.MergeDescriptions(claimsData, scoredData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection merge-descriptions: %s\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outPath, append(merged, '\n'), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "resection merge-descriptions: %s\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "resection merge-descriptions: wrote %s\n", *outPath)
}
