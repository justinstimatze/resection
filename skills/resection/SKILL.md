---
name: resection
description: Run a two-cold-agent drift check between a project's stated vision (HANDOFF.md, README, design docs) and its actual implementation, surfacing where the two accounts disagree as the finding. Invoke manually on a project you suspect has drifted from its own vision, or when resection's own SessionStart hook injects an instruction to run it after the Nth /compact. Defaults to the current project (cwd); pass a path to check a different one.
---

# resection

Two independently-briefed cold agents each produce an account of "what this
project is/does" — one from only the vision docs, one from only the actual
implementation — and a deterministic diff surfaces where they disagree.
Disagreement is the finding, not a summary. See `docs/design.md` in the
resection repo for the full design rationale.

Target project defaults to the current working directory; if the invoker
named a different path, use that instead everywhere below.

## Steps

1. **Gather vision material.** Glob the target project for: `HANDOFF.md`,
   `README.md`, `DESIGN.md`, `ARCHITECTURE.md`, `VISION.md`, `docs/**/*.md`,
   `decisions/**/*.md`, `adr/**/*.md`. Read each and concatenate them
   **verbatim** — do not summarize or paraphrase; summarizing here would
   let your own (possibly drifted) judgment leak into what the cold agent
   sees, which defeats the point. Cap at ~80KB total; if over, truncate the
   least-central files first and note the truncation in the final report.

2. **Get a run directory.** `resection rundir --project <target-project-root>`
   (or just run `resection rundir` from within the target directory, which
   defaults to cwd). Pass a project *root path* either way — the CLI
   resolves it internally the same way a PostCompact/SessionStart hook
   resolves its own cwd, so this always lands in the same project bucket a
   real hook invocation for that project would use. Write everything below
   into this directory.

3. **Dispatch the vision-reader, enumerate mode.** `Agent(subagent_type:
   "resection-vision-reader")` with a prompt that inlines the concatenated
   vision material and instructs "Enumerate" mode per the agent's own
   instructions. Extract the first fenced json array from its response,
   write it to `<rundir>/claims-raw.json`.

4. **Assign IDs.** `resection assign-ids --in <rundir>/claims-raw.json
   --out <rundir>/claims.json`.

5. **Dispatch the vision-reader again, score mode.** New `Agent()` call
   (same agent type, fresh dispatch — not a continuation of step 3's
   conversation), inlining the same vision material plus the numbered
   `claims.json` list, instructing "Score" mode. Extract the fenced json
   object, write to `<rundir>/vision-raw.json`, then run `resection
   merge-descriptions --claims <rundir>/claims.json --in
   <rundir>/vision-raw.json --out <rundir>/vision.json` — the agent isn't
   asked to reproduce each claim's description, so this fills it back in
   from claims.json before validation ever sees the file. This step also
   errors if the agent dropped a claim entirely rather than scoring every
   one it was given — see step 7 for how to handle that failure.

6. **Dispatch the implementation-reader.** `Agent(subagent_type:
   "resection-implementation-reader")` with a prompt naming the target
   project's root path and inlining `claims.json` (the bare claim list,
   not the vision agent's scores — it must not see those). Instruct it to
   score each claim per its own instructions. Extract the fenced json
   object, write to `<rundir>/impl-raw.json`, then run the same `resection
   merge-descriptions --claims <rundir>/claims.json --in
   <rundir>/impl-raw.json --out <rundir>/impl.json` step.

7. **Validate both.** `resection validate --role vision --in
   <rundir>/vision.json` and `resection validate --role implementation
   --in <rundir>/impl.json`. The same one-bounded-repair-attempt protocol
   applies whether the failure surfaces from `merge-descriptions` (a
   dropped claim, step 5/6) or from `validate` (a schema violation): on
   failure, re-dispatch *the same subagent* with the exact error and "fix
   only the JSON, same format" — not a fresh cold read — then re-run that
   role's merge-descriptions step against the repaired raw output before
   validating again. One repair attempt per report. A second failure on
   the same report stops here: report the hard error to the user rather
   than continuing on bad data.

8. **Diff.** `resection diff --vision <rundir>/vision.json --impl
   <rundir>/impl.json --out <rundir>/diff.json`. If this errors — a
   claim-ID mismatch, a wrong `role`, or a project mismatch between the
   two reports — that's a pipeline bug (a file handed to the wrong flag,
   or the implementation-reader altering the list it was given), not a
   finding — stop and report it, don't paper over it. Otherwise its stdout
   already lists the flagged (disagreeing) rows in human-readable form.

9. **Adjudicate flagged rows.** For each flagged row printed in step 8,
   decide from what you've already seen this turn: is this real drift, or
   two legitimate differing framings of the same thing (e.g. the vision
   side calls something "done" at a coarser grain than the implementation
   side does)?

9a. **Verify every headline claim, not just the flagged ones.** This step
    exists because of a real gap: a headline claim where both sides
    independently agree can never appear in step 8's output — the diff
    only ever prints disagreements, so an agreed-but-wrong headline claim
    is invisible to everything upstream of this step. It's also exactly
    the shape a same-labeled-but-easier substitute produces (both
    readers label-match past the same gap and land on an identical,
    identically-wrong status) — the one case this whole mechanism exists
    to catch. For every claim marked `headline: true` in claims.json,
    matched or flagged, read that claim's row in both vision.json and
    impl.json directly (not diff.json) and render an explicit verdict:
    does either side's `falsification_criterion` actually name a
    concrete substitute and explain why the evidence rules it out, or
    does it just restate the label match ("a function named for this
    exists" is not a falsification check)? This is a real read of the
    prose, not a presence check — `resection validate` already enforces
    that the field is non-empty; your job here is judging whether its
    content is substantive.

10. **Write the report.** `<rundir>/report.md`: the full claim table (all
    rows, not just flagged ones), a clearly separated section with your
    adjudicated verdict on each flagged row and why, and a separate
    "Headline claims" section with your step 9a verdict for every
    headline claim — including matched ones, since those never appear
    anywhere else in the pipeline's output. Include one caveat line: the
    claim list was authored by the same vision-reader dispatch that
    scored the vision side (step 3 and step 5 are the same agent type,
    not independently produced), so treat vision-side "done" verdicts
    with slightly more skepticism than implementation-side ones — this is
    a known, accepted v1 tradeoff (see docs/design.md), not an oversight.

11. **Summarize to the user.** State the flagged count, name the headline
    claims among them first, and give your adjudicated read on each — plus
    any headline claim from step 9a whose falsification_criterion didn't
    hold up even though it matched. Point at `<rundir>/report.md` for the
    full table rather than pasting it whole.
