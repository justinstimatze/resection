# Security Policy

resection is experimental: v1 runs, but has not been hardened for adversarial input. See
`README.md`'s Status section for what's actually been run.

## Only run this against a project you trust

resection's implementation-reader agent is given real `Read`/`Grep`/`Glob`/`Bash` access to
the target project's checkout, and its own instructions (`agents/resection-implementation-reader.md`)
tell it to build the project and run its test suite as part of scoring claims — that's real
code execution, by design, not an incidental capability. Separately, the orchestrating skill
(`skills/resection/SKILL.md` step 1) reads the target project's own vision documents verbatim
and inlines them into a prompt sent to the vision-reader agent and to the orchestrating
session itself, which is a direct prompt-injection channel from anything those documents
contain. Bash is restricted to inspection only at the prompt level (no write, no install, no
commit, no push) — that is a model-level convention the implementation-reader agent is
instructed to follow, not something resection enforces structurally.

**Do not point resection at a repository whose contents you don't trust.** It will execute
that repository's build and test scripts, and it will feed that repository's prose to an
agent with real tool access.

## Reporting

If you discover a security vulnerability, please email
justin@justinstimatze.com directly rather than opening a public issue or PR.

I'll acknowledge receipt within 7 days and aim to provide an initial
assessment within 30 days. We can coordinate on a disclosure timeline —
defaulting to 90 days from initial report unless circumstances warrant
otherwise.

Thanks for helping keep this project and its users safe.
