# Design

This document explains why resection is built the way it is: the failure it targets, the
mechanism it uses, the alternatives it rejected, and the limits it accepts rather than
oversells. See [README.md](../README.md) for what the tool does and how to run it.

## The problem

In a long-running Claude Code session, the model's working picture of a project and the
project's actual state drift apart. Context gets summarized, summarized again, and each
summary loses detail no single turn would have dropped on its own. Neither the human nor
the model reliably notices the drift once it has happened — there's no natural moment where
either party re-derives "what is this project, actually" from scratch and checks it against
what they've been assuming.

## Why not self-report

[pellicle](https://github.com/justinstimatze/pellicle) drops a Context / Contributions /
Consequences writeup into the transcript at the end of every turn.

The difference: pellicle is self-report — the same agent, in the same possibly-already-drifted
context, narrating itself. If the drift already corrupted the agent's picture, the self-report
inherits that exact blind spot. Two cold agents, reading disjoint material with no access to
the session that drifted, can't inherit that specific blind spot. That's the argument for the
extra cost of running two of them instead of one self-report.

## Three failure categories, only one of which resection solves

**Session drift** — the working session's picture diverges from the project's real state.
This is what resection's independence buys protection from.

**Incomplete documentation** — both a cold vision-reader and a cold implementation-reader only
see what got written down. If the vision was never captured accurately, or the code has real
behavior no doc mentions, both readers miss the same thing. Independence does not fix this.

**Accurate-but-unflagged scope narrowing** — a third, distinct failure, found in a sibling
project's own code-clone detector: its docs correctly described a real, measured Type-4 (behavioral)
clone detector, but the mechanism that actually gated CI was a much easier Type-1–3 check
"by construction," with the true Type-4 mechanism living in a separate, non-blocking,
manually-invoked generator. The documentation was honest about the split; reading the headline
claim alone, without registering the gate/generator split, produced exactly the wrong model of
what was actually enforced. This is worse than incomplete documentation, because "read more
carefully" doesn't fix it — it requires already knowing which single fact the claim actually
rests on, so you know to be suspicious of that one.

It breaks the typed-schema-diff design in a specific way: both cold agents could independently
read accurate material and both label-match a headline claim to "done" — a function named for
the claim exists, a README section is titled for it — without either checking whether the
implementation actually satisfies the claim's real technical bar, rather than an easier
same-labeled substitute. That produces agreement, and therefore no flagged disagreement, on
exactly the claim that mattered most. The claim schema's `headline` and `falsification_criterion`
fields exist specifically to counter this (see [README.md](../README.md#what-each-side-can-and-cant-catch)
for the field-level detail and its own stated limits).

## Reconciliation: a deterministic diff, not a third opinion

The reconciliation step follows one rule: ensemble agreement is never the trust signal.
Generating two independent accounts and comparing them only works if the comparison itself is
mechanical, not a third LLM's free-form judgment layered on top of two other LLMs' output —
that would just add a third opinion instead of a check.

Applied here: both cold agents fill the same typed schema, one row per claim, status one of
`done` / `partial` / `not-started` / `contradicted` plus an evidence pointer. The two schemas are
diffed field-by-field in code — no LLM in that step at all. An LLM pass is spent only on the rows
the diff flags as disagreeing, deciding whether each is real drift or two legitimate framings of
the same thing. The closest classical relative is Gail Murphy and David Notkin's *reflexion
models* work (mid-1990s) — build a high-level model, extract a source model from the actual
code, compute the divergence — applied here to LLM-produced accounts instead of mechanically
extracted ones.

## Trigger condition

Structural, with a manual override kept live alongside it — not manual-only, and not
shape-detection. A manual-recognition trigger works when the human recognizing the shape isn't
the one who might be blind — mid-task, they're gating cost, not diagnosing their own drift. Here
the premise is that neither party reliably notices drift once it's happened, so a trigger that
depends on either of them recognizing "this looks like a drift moment" reproduces the exact
failure the tool exists to catch.

Chosen: fire on every Nth `/compact`, not every one. A plain per-compact trigger lands at the
right moment — context is already being rewritten and made lossy right there — but pays two full
cold-agent dispatches every time, too expensive at compact frequency. N is not fixed by design;
it's picked empirically once the tool has run enough times to produce a real cost-per-run number
to weigh against how often flagged rows turn out to matter.

Manual invocation stays available regardless of N. This doesn't reintroduce the noticing problem
— it's a second, cheaper-to-reach path on top of the structural one, not a replacement for it.

## v1 limits, stated plainly

- **The vision-side enumerate and score passes run under the same agent type.** Claims are
  enumerated by the same vision-reader dispatch that later scores them — not two fully
  independent productions. Treat vision-side `done` verdicts with somewhat more skepticism than
  implementation-side ones; this is a deliberate v1 tradeoff, tracked here rather than hidden,
  and a split-pass version is the natural next experiment if the vision side's
  flagged-disagreement rate looks suspiciously low.
- **Evidence quality is checked only for presence.** The schema requires a non-empty evidence
  string; a confident-sounding sentence with no real substance behind it still validates.
- **Resection finds drift between what's written and what's true — not drift between what's true
  and what should have been written but never was.** A real incident can be discovered entirely
  by empirical testing, with no vision document ever asserting the fact the code contradicted.
  Nothing in this design catches that shape of gap; it isn't drift from a claim, because there
  was no claim.
- **The closed-book guarantee depends on a non-empty tools list, not an empty one.** Claude
  Code treats a subagent's `tools: []` as equivalent to omitting the field entirely, granting
  the full default tool set instead of zero. `resection-vision-reader.md` lists a single real,
  file-and-network-free tool (`TaskStop`) instead, so the restriction is a platform-enforced
  tool grant rather than the agent choosing to ignore tools it technically has.
