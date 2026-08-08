# tests/security

Per `CLAUDE.md`'s Testing Layout: cross-service fraud-bypass attempts and
auth-boundary checks, run against each service's OpenAPI spec.

**Status: empty, deliberately deferred.** `docs/openapi.yaml` has no
content yet, so there's no spec to drive HTTP-level tests against. This
directory stays empty until a service publishes a real OpenAPI spec.

## Interim coverage (planned, not yet written)

The harness audit found a real gap worth a service-level test in the
meantime: `services/ledger-service`'s `InsertTransaction` has **no
authentication or account-ownership check at all** — any caller can post
entries against any `account_id`, and a transaction can "self-balance" on
a single account (debit A 1000 / credit A 1000) without moving anything
real between parties. A characterization test for this
(`internal/ledger/repository_test.go`) is intended to be written next, in
a mentor-mode session (ledger-service business code, not this harness
change) — it won't fix the gap, just make the current risky behavior
explicit and regression-tested so a future auth layer has something
concrete to change.
