# Coding Standards Design

**Date:** 2026-08-01
**Status:** Approved for implementation

## Purpose

SettleGuard currently has no repo-wide coding standards document. `CLAUDE.md`
covers *what* stack each service uses (Stack section), *how* tests are
organized (Testing Layout), and *how* branches/commits/merges work (Git
Workflow) — but nothing about naming conventions, formatting, or error
handling patterns within the code itself. As real implementation work
starts (ledger-service is the first service being built), these decisions
need to be made explicit rather than left to be improvised per-file.

This spec defines a new `docs/CODING_STANDARDS.md`, self-contained (not
just pointers to external style guides), covering two areas: naming &
formatting, and error handling patterns — for both Go and Python, since
the repo is polyglot (Go for accounts-service/ledger-service/
settlement-engine, Python for notification-service and all of `tests/`).

Explicitly out of scope for this pass (may become their own
spec/plan later):
- Lint tooling setup (golangci-lint, ruff/mypy config files)
- CI enforcement (no `.github/workflows` exists yet)
- PR / code review checklist

## File Location & Structure

New file: `docs/CODING_STANDARDS.md`, with two top-level sections —
**Naming & Formatting** and **Error Handling Patterns** — each split into
a Go subsection and a Python subsection (the two languages have different
idioms, so rules are not unified across them).

Every rule is written as a ✅ **Nên** / ❌ **Không nên** pair with a code
example drawn from this project's actual domain (ledger entries, account
IDs, money amounts, risk scoring) rather than generic `foo`/`bar` examples
— the target reader is re-learning both languages, so concrete,
domain-grounded examples matter more than abstract style-guide prose.

`CLAUDE.md` gets one new line under its **Stack** section pointing to
`docs/CODING_STANDARDS.md`, following the same pattern it already uses to
point to `docs/PROJECT_CHARTER.md`.

## Content: Naming & Formatting

**Go:**
- Package names: short, lowercase, no underscores (`ledger`, not
  `ledger_service`)
- Formatting via `gofmt`/`goimports` is non-negotiable — this is Go's
  fixed baseline, not an optional lint choice
- Imports grouped in three blocks, blank-line separated: standard
  library → third-party → internal package (matches the pattern already
  used in the ledger-service-mvp plan's code)
- Acronyms keep consistent case: `AccountID`/`TransactionID`, never
  `AccountId`/`TransactionId`
- Sentinel error variables are always prefixed `Err`:
  `ErrUnbalancedTransaction`, not `UnbalancedTransactionError`
- Test functions: `TestXxx`, test files: `_test.go`

**Python:**
- `snake_case` for functions/variables/filenames, `PascalCase` for
  classes
- Formatting via `black`, 88-character line length
- Imports grouped in the same three blocks as Go: standard library →
  third-party → local module
- Type hints encouraged on function signatures, especially anywhere
  touching money or risk scoring (`def score_transaction(amount: int) ->
  RiskScore:`)

## Content: Error Handling Patterns

**Go:**
- Return `error` as the final return value; don't use `panic` for
  expected failure conditions (bad input, DB errors) — panic is reserved
  for unrecoverable programmer errors
- Known, caller-checkable domain failures get sentinel errors
  (`errors.New`), declared near the domain type they belong to — the
  pattern already established by `ErrUnbalancedTransaction`,
  `ErrInvalidAmount`, `ErrInvalidDirection`, `ErrNoEntries` in the
  ledger-service plan
- Wrap errors crossing layers with `fmt.Errorf("...: %w", err)`, never
  `%v` — `%w` preserves the original error so callers can still
  `errors.Is`/`errors.As` it
- Never compare errors by string (`err.Error() == "..."`); always
  `errors.Is`
- Never silently discard an error (`_ = err`) without an explicit comment
  justifying it
- At the HTTP boundary (`api/handlers.go`), map domain errors to HTTP
  status codes explicitly via `errors.Is` (as in the plan's
  `CreateTransaction` handler mapping `ErrUnbalancedTransaction` → 422);
  never let an internal error's raw message leak into the response

**Python:**
- Use exceptions for genuinely exceptional conditions, not return
  codes/`None` as an error signal
- Domain-specific failures get their own exception class (e.g.
  `class RiskScoringError(Exception)`, once notification-service/tests
  code exists), rather than raising a generic built-in exception when a
  domain-specific one would be clearer
- When wrapping/re-raising, use `raise NewError(...) from err` to
  preserve the original traceback — Python's equivalent of Go's `%w`
- Only catch exception types you can meaningfully handle; avoid bare
  `except Exception:` except at an explicit outer boundary (e.g. CI
  failure triage tooling) with a comment explaining why

## Testing

Not applicable — this is a documentation-only change (no executable code
to test). Verification is a self-review pass on the written doc for
consistency with the examples already established in
`docs/superpowers/plans/2026-07-29-ledger-service-mvp.md`.
