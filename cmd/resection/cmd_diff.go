package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/resection/internal/claim"
)

// cmdDiff implements `resection diff --vision F --impl F --out F`. It
// requires the two reports to actually be a vision/implementation pair
// for the same project referencing the same claim-ID set (claim.Diff
// errors otherwise — a pipeline bug, not a finding) and prints only the
// flagged rows to stdout in human-readable form, so the orchestrating
// skill sees them directly in its own transcript for inline adjudication
// (v1 scope — see skills/resection/SKILL.md step 9).
func cmdDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	visionPath := fs.String("vision", "", "path to the vision Report JSON")
	implPath := fs.String("impl", "", "path to the implementation Report JSON")
	outPath := fs.String("out", "", "path to write the full DiffResult JSON")
	_ = fs.Parse(args)

	if *visionPath == "" || *implPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: resection diff --vision F --impl F --out F")
		os.Exit(2)
	}

	vision, err := readReport(*visionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection diff: reading vision report: %s\n", err)
		os.Exit(1)
	}
	impl, err := readReport(*implPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection diff: reading impl report: %s\n", err)
		os.Exit(1)
	}

	result, err := claim.Diff(vision, impl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection diff: %s\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection diff: marshal: %s\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "resection diff: writing %s: %s\n", *outPath, err)
		os.Exit(1)
	}

	if result.FlaggedCount == 0 {
		fmt.Printf("resection diff: %d claims, no disagreement\n", len(result.Rows))
		return
	}
	fmt.Printf("resection diff: %d of %d claims disagree\n\n", result.FlaggedCount, len(result.Rows))
	for _, row := range result.Rows {
		if row.Match {
			continue
		}
		headline := ""
		if row.Headline {
			headline = " [headline]"
		}
		fmt.Printf("- %s%s: %s\n", row.ID, headline, row.Description)
		fmt.Printf("  vision: %s (%s)\n", row.VisionStatus, row.VisionEvidence)
		fmt.Printf("  impl:   %s (%s)\n", row.ImplStatus, row.ImplEvidence)
	}
}

func readReport(path string) (claim.Report, error) {
	var r claim.Report
	data, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(data, &r)
	return r, err
}
