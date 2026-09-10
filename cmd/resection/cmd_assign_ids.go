package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/justinstimatze/resection/internal/claim"
)

// cmdAssignIDs implements `resection assign-ids --in FILE [--out FILE]`.
// It numbers a bare JSON array of {"description","headline"} objects
// (the vision-reader's "enumerate" output) as claim-01, claim-02, ... in
// the order given — IDs are assigned here, in Go, never by either LLM,
// so two independent outputs can't invent conflicting IDs for "the same"
// claim.
func cmdAssignIDs(args []string) {
	fs := flag.NewFlagSet("assign-ids", flag.ExitOnError)
	inPath := fs.String("in", "", "path to the bare claim-stub JSON array")
	outPath := fs.String("out", "", "path to write the IDed stub array (default: stdout)")
	_ = fs.Parse(args)

	if *inPath == "" {
		fmt.Fprintln(os.Stderr, "usage: resection assign-ids --in FILE [--out FILE]")
		os.Exit(2)
	}

	data, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection assign-ids: %s\n", err)
		os.Exit(1)
	}
	var stubs []claim.ClaimStub
	if err := json.Unmarshal(data, &stubs); err != nil {
		fmt.Fprintf(os.Stderr, "resection assign-ids: invalid JSON: %s\n", err)
		os.Exit(1)
	}
	if len(stubs) == 0 {
		fmt.Fprintln(os.Stderr, "resection assign-ids: input array is empty")
		os.Exit(1)
	}
	for i := range stubs {
		stubs[i].ID = fmt.Sprintf("claim-%02d", i+1)
	}

	out, err := json.MarshalIndent(stubs, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resection assign-ids: marshal: %s\n", err)
		os.Exit(1)
	}
	if *outPath == "" {
		fmt.Println(string(out))
		return
	}
	if err := os.WriteFile(*outPath, out, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "resection assign-ids: writing %s: %s\n", *outPath, err)
		os.Exit(1)
	}
}
