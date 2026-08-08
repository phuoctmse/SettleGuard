# SettleGuard Project Charter

## Vision

SettleGuard is a B2B platform that tracks and settles financial obligations
between parties on top of external payment rails — it never moves real money
itself — while actively guarding the settlement process with rule-based and
ML-based risk scoring. Client businesses integrate via API to settle
payments/payouts between their own users. SettleGuard's own mobile app lets
end-users (or the client's ops team) track settlement status and see
fraud/risk alerts in real time.

## In Scope (v1)

- Single-currency obligation tracking and settlement (accounts, ledger,
  settlement-engine)
- Real-time rule + ML risk scoring on transactions as they arrive
- Batched settlement runs that finalize risk-cleared obligations
- Notifications (push/email) for settlement events and risk holds
- Mobile app for settlement/alert visibility
- Cloud deployment (Kubernetes + Terraform)

## Out of Scope (v1) — Non-Goals

- Direct movement of real funds (no custody, no payout execution — delegated
  to an external payment processor)
- Multi-currency / FX
- On-premise deployment

For the full architecture, domain model, and tech stack, see
[docs/superpowers/specs/2026-07-29-project-charter-design.md](superpowers/specs/2026-07-29-project-charter-design.md).
