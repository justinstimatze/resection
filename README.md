# resection

**Experimental.** See [Status](#status) for what's actually been run before pointing this at
something you depend on.

> A vision-vs-implementation drift audit: two cold agents produce independent accounts of a
> project, and a deterministic diff surfaces where they disagree.

Long-running Claude Code sessions drift: the model's working picture of a project and the
project's actual state diverge, and neither the human nor the model reliably notices once it's
happened. resection runs two agents that never see each other's output or the working session's
context — one reading only the project's stated vision, one reading only its actual
implementation — scores the same set of claims independently, and diffs the two scorecards in
code. Disagreement is the finding — a summary of either side alone says nothing.

## What resection does

1. A closed-book agent (a single inert tool, not an empty tools list — see
   [How it works](#how-it-works)) reads the project's vision documents, verbatim and in full,
   and enumerates 8-20 falsifiable present-tense claims about what the project is or does.
2. The same agent type, in a fresh dispatch, scores each claim `done` / `partial` /
   `not-started` / `contradicted` against the pasted material alone.
3. A second agent, with real read/grep/glob/bash access to the checked-out project and instructed
   not to read the vision documents, scores the identical claim list against the actual code,
   tests, and observed behavior. Unlike the vision-reader's tool grant, this side's isolation is a
   prompt convention, not a structural one — see [What each side can and can't
   catch](#what-each-side-can-and-cant-catch).
4. A deterministic Go function diffs the two scorecards field-by-field. No LLM judges agreement —
   the diff is a plain equality check on a typed `status` field.
5. An LLM pass runs on every row the diff flagged, plus every claim marked `headline: true`
   regardless of whether it was flagged — see [What each side can and can't
   catch](#what-each-side-can-and-cant-catch) for why matched headline claims need a look too.

See [docs/design.md](docs/design.md) for why this shape, what it rejected, and the failure
categories it does and doesn't cover.

## How it works

```
resection-vision-reader (tools: [TaskStop])  resection-implementation-reader (tools: [Read, Grep, Glob, Bash])
        │                                              │
        │ reads pasted vision docs only                │ reads the real checkout only
        ▼                                              ▼
   vision.json  ──────────────┐          ┌────────── impl.json
   (Report, one row per claim)│          │(Report, one row per claim)
                               ▼          ▼
                          resection diff
                          (Go, field-by-field status equality)
                               │
                               ▼
                       flagged rows only
                               │
                               ▼
                 orchestrating session adjudicates
```

Claim IDs are assigned once, in Go, by `resection assign-ids` — never by either agent, so the two
sides can't invent conflicting IDs for what should be the same claim. In practice, both agents
also tend to drop or reword each claim's `description` text when they echo it back — a
reliability problem, not a design choice — so `resection merge-descriptions` runs on every raw
output before validation and re-attaches the original `description` by ID, closing that gap
without asking either agent to get the reproduction right.

A scored claim looks like this:

```json
{"id": "claim-03", "description": "...", "headline": true, "status": "contradicted",
 "evidence": "config/env.mjs:30 — retired 2026-07-28, but router.js:12 still targets it",
 "falsification_criterion": "an easier substitute would keep the label without clearing the bar; here it doesn't clear it at all"}
```

A flagged diff row carries both sides' `status` and `evidence` side by side — see
`internal/claim/diff.go`'s `DiffRow` type for the full shape.

## Install

```sh
git clone https://github.com/justinstimatze/resection
cd resection
make build
./resection install
```

`resection install` symlinks the two agent types and the orchestrating skill into `~/.claude/`.
It does not wire the automatic trigger — pass `--with-hooks` once a real N has been chosen (see
[Configuration](#configuration)). `resection uninstall` reverses it; `--keep-data` preserves run
history. `resection show [--project PATH]` lists past runs for a project; `resection status
[--project PATH]` reports the compact counter and the tail of `hook.log`.

## Manual invocation

Type `/resection` in a Claude Code session, optionally with a path to a different project, at any
time. The automatic trigger below runs this identical skill on a schedule; manual invocation is the
same mechanism reachable on demand, always available regardless of whether the trigger is wired.

## Configuration

| Setting | Where | Default |
|---|---|---|
| `N` (compacts between automatic firings) | `RESECTION_N` env var | `8` — a placeholder, not a considered choice |
| Automatic trigger | `resection install --with-hooks` | off |
| Vision-document glob | `skills/resection/SKILL.md` step 1 | `HANDOFF.md`, `README.md`, `DESIGN.md`, `ARCHITECTURE.md`, `VISION.md`, `SECURITY.md`, `docs/**/*.md`, `decisions/**/*.md`, `adr/**/*.md`, capped ~80KB |

`N` has no considered value yet — it's picked empirically once a project has run resection
enough times to produce a real cost-per-run figure to weigh against how often flagged rows turn
out to matter. Manual invocation works fully with the automatic trigger off.

## What each side can and can't catch

The claim schema carries `headline` and `falsification_criterion` fields specifically because two
independently-cold agents can both label-match a claim to `done` — a function named for it exists,
a README section is titled for it — without either checking whether the implementation clears the
claim's real technical bar rather than an easier same-labeled substitute. A headline claim's
`falsification_criterion` is required to state what that easier substitute would look like and
whether the evidence actually rules it out.

`resection validate` mechanically enforces that a headline claim can't ship with an empty
`falsification_criterion` — but presence isn't substance. A matched headline claim (both sides
say `done`) never appears in `resection diff`'s own output, since the diff only ever prints
disagreements — exactly the case where two independently-cold agents could both label-match past
the same gap and never get caught. So the skill's own adjudication step reads every headline
claim's `falsification_criterion` directly, matched or not, and judges whether it actually names
a substitute and rules it out, rather than restating the label match (see
[docs/design.md](docs/design.md#reconciliation-a-deterministic-diff-not-a-third-opinion)). It
cannot enforce that a non-headline claim's `evidence` field points at something real rather than
a plausible-sounding assertion — evidence quality there is checked only for presence, so a
confident-sounding sentence with nothing behind it still validates.

The implementation-reader's independence has the same shape of gap `SECURITY.md` already
discloses for its Bash access: real tools, prompt-restricted rather than structurally restricted.
It's told not to read the project's vision documents, and its own checkout contains them —
nothing stops it from opening `README.md` the way `tools: []` used to fail to stop the
vision-reader from reaching everything.

resection catches drift between what's written and what's true. It does not catch drift between
what's true and what should have been written but never was — a real bug can be found entirely by
empirical testing, with no vision document ever asserting the fact the code contradicted, and
nothing in this design flags a claim that was never made.

## Status

**What's run so far** (as of 2026-09-10 — this list only grows; check `git log -- README.md`
for anything newer):

- Four full self-audits against this repository. The first two, together, found: a stale
  status claim, a project-hash inconsistency between the CLI and the hooks, and a fragile
  `uninstall` matcher (all three fixed in `b3473fd`); and, independently on both runs, both
  agents dropping or rewording a claim's `description` on echo (fixed in `ea2ba2a` — see
  [How it works](#how-it-works)). The third caught this Status section undercounting real usage.
  The fourth caught three more: the implementation-reader's vision-doc access being convention
  rather than structural (see [What each side can and can't
  catch](#what-each-side-can-and-cant-catch)), a "the LLM pass runs only on flagged rows" claim
  that step 9a had already made false, and the wrong "organic" attribution in the bullet below.
- One deliberate backtest against a real external (private) project's own historical incident.
  8 of 18 claims flagged, including two
  independently-corroborated real findings (a removed access-control gate a doc still
  described as live; a config file a doc's own section title claimed didn't exist, added the
  same date the doc claims to have verified everything against live systems) and one claim
  where both sides independently reached `contradicted` from unrelated evidence — agreement on
  real drift, which correctly produces no flag. This run predates `resection rundir`'s current
  storage path, so it isn't in this tool's own persisted run history — true, but currently
  unverifiable from the project's own state, which is itself a real gap.
- resection has also run against two other projects on this host (not deliberate backtests, not
  this project's to name in its own docs): 3 of 16 claims flagged in one, all headline; 1 of 17
  in the other, headline. Neither adjudicated here. One project's own counter confirms the
  automatic trigger actually fired; the other's counter shows zero fires despite having a
  completed run, so that one was invoked manually — a completed run doesn't by itself mean the
  trigger caused it, and only the counter file settles which.

**What works:** the claim schema, the deterministic diff, both agent types, the orchestrating
skill, install/uninstall, `merge-descriptions`.

**What's open:** the automatic Nth-compact trigger is wired but its `N` is unpicked; the v1
`headline`/`falsification_criterion` scheme is the only defense against label-matching, and its
own limits are stated above rather than papered over; a [gemot](https://github.com/justinstimatze/gemot)-based adjudication layer (multi-agent
deliberation, in place of the orchestrating session judging flagged rows alone) for flagged
rows is deferred until there's evidence the schema/diff loop is useful without it.

## License

MIT. See `LICENSE`.
