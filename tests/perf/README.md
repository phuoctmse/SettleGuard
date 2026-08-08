# tests/perf

Per `CLAUDE.md`'s Testing Layout: load/perf tests for settlement-engine's
real-time scoring path and batch settlement throughput.

**Status: empty, deliberately deferred.** `settlement-engine` has no code
yet — there's nothing to load-test. This directory stays empty until that
service exists.

## Interim coverage (planned, not yet written)

A throughput baseline for the one write path that exists today is
intended to be added next: `BenchmarkRepository_InsertTransaction` in
`services/ledger-service/internal/ledger/repository_bench_test.go`,
written in a mentor-mode session (ledger-service business code, not this
harness change). When settlement-engine starts batching "all
risk-cleared entries" on a schedule, this benchmark becomes the reference
point for how fast ledger-service can actually feed it.
