# `resection-implementation-reader` never shows its own schema — two independent dispatches both wrote `verdict` instead of `status`

Found running resection against keyway, the first real target since resection's own
v0.1.1 release.

## What happened

The implementation-score step failed `resection validate` twice in a row. First:
`schema_version: "1.0"` instead of the integer `1`. Repaired that (one exact-error
re-dispatch, per the skill's own protocol), re-validated, and hit a second failure:
every one of 18 claims had an empty `status` field, because the agent had written
`"verdict": "done"` instead of `"status": "done"` — consistently, not a one-off typo.

Per the skill's own rule ("one repair attempt per report... a second failure stops
here"), the run stopped there rather than taking a third swing at the same report.
That's the protocol working as designed, not a gap in it — worth saying plainly since
the rest of this note is about something that did need fixing.

## Root cause

`resection-vision-reader.md` spells out the literal field name with a worked JSON
example (lines 61-71): `{"id": "claim-01", ..., "status": "done", ...}`, plus an
explicit "`schema_version` is the JSON integer `1`, not the string `"1"` or `"1.0"`"
warning. `resection-implementation-reader.md` had neither — it just said "matching the
Report schema" and left the agent to infer the actual keys. Two independent cold
dispatches (the original, and the one sent back for the schema_version repair, which
only got told to fix that one field) both reached for `verdict` over `status`, because
nothing in that prompt ever showed the real key. Not a fluke; a gap in the prompt.

## Fix (already committed, `83b10de`)

Added the same worked-example treatment `resection-vision-reader.md` already has:
the integer warning, a fenced JSON example with `"status"` shown verbatim, and a note
on the optional `notes` field (see below). `go build ./...` and `go test ./...` both
pass; this is a markdown-only change so neither exercises it directly, but nothing else
in the repo broke.

## Loose end, not fixed — your call

The implementation-reader's actual output organically included a `notes` field on most
claims (`internal/claim/schema.go:67`, `json:"notes,omitempty"` — a real, supported
field) despite nothing in *either* agent's prompt ever mentioning it exists. It turned
out genuinely useful — e.g. flagging that a claim describes a past point in the
project's own history rather than current state, which `status` alone can't express.
I documented it in the implementation-reader's prompt as part of this fix, since that's
the one that broke. `resection-vision-reader.md` still doesn't mention `notes` at all,
which is an asymmetry worth closing for the same reason the status-field gap was worth
closing — I didn't touch it since it wasn't the thing that actually failed this run, and
the request was for this fix specifically.

## Context

Everything else in the pipeline behaved: `assign-ids`, both `merge-descriptions` calls,
`diff`, the vision side's own repair cycle (it dropped a `falsification_criterion` on one
headline claim, fixed cleanly in one re-dispatch). The one real substantive finding that
survived the implementation-report failure anyway — because I had the raw (if
unvalidated) JSON in hand — was itself a good one: `keyway`'s README claims `keyway`
"emits nothing if \[the model field] is missing," which the implementation-reader caught
overclaiming by actually building the binary and piping in a payload with no `model`
field at all against a tier directory with only `_base.md` — it printed the base content
anyway, because base-tier emission doesn't gate on model at all. That's exactly the kind
of thing this tool is supposed to catch, and it caught it on only its second real run.

One more thing worth naming for whoever reads this later, not a resection bug: this
session's harness flagged the implementation-reader's returned JSON as "matched
instruction-shaped pattern(s): settings-json" on every dispatch. That's the harness
reacting to a literal `.claude/settings.json` snippet quoted verbatim inside the
evidence text (quoting verbatim is correct and required here) — not an actual injection
attempt. Expect this on any project whose vision docs embed JSON examples; it's noise
specific to quoting JSON back, not a resection problem to fix.

Reported from a session on keyway (github.com/justinstimatze/keyway), 2026-09-11.
