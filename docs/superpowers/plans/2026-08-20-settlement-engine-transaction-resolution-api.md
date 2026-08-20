# settlement-engine: held-transaction resolution + read API

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps
> use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the gap identified in
`docs/superpowers/specs/2026-08-19-mobile-app-design.md` §4: `held` is
currently a dead-end status (comment in
`internal/settlement/transaction_repository.go`: *"terminal-for-now state
(no resolution path yet in this MVP)"*), and settlement-engine has no HTTP
surface beyond `GET /health`. This plan adds exactly what
`mobile-app`'s Held Transactions and Settlements screens need: list/get
transactions, approve/reject a held transaction, list/get settlements —
nothing else (no pagination beyond what's specified, no auth, no bulk
actions).

**Branch:** `service/settlement-engine` (extends the existing merged MVP —
not a new service, so it does not get its own `service/` branch per the
repo's git workflow).

**Blocks:** `mobile-app` plan
(`docs/superpowers/plans/2026-08-19-mobile-app-mvp.md`) Tasks 5 and 6.

## Design decisions

1. **New terminal status `rejected`.** `held` → `approve` → back into the
   normal flow (`pending_settlement`, picked up by the next
   `RunBatch` exactly like a `pass` decision would have been — no separate
   "approved" status, since post-approval a transaction behaves identically
   to one that passed risk-scoring outright). `held` → `reject` →
   `rejected`, which — like `settled` — never transitions again. This
   keeps the state machine to 4 statuses total instead of inventing a
   parallel "approved" status that would need its own handling everywhere
   `pending_settlement` is already handled.
2. **Approve/reject publish a new event, `transaction.resolved`.** Every
   other state transition in this service that matters to the rest of the
   system already publishes one (`transaction.risk-scored`,
   `settlement.finalized`); a human overriding a risk hold is at least as
   significant an audit event as either of those, and it's the kind of
   thing `notification-service` (or a future audit/compliance consumer)
   plausibly wants to know about. Payload: `{transaction_id, resolution:
   "approved"|"rejected", resolved_at}`. Published via the same outbox
   pattern (`outbox_events` table, existing `Relay`) already used for the
   other two events — no new infrastructure. Subject `transaction.resolved`
   is added to the `SETTLEMENT_EVENTS` stream's subject list (alongside
   the existing `settlement.>` and `transaction.risk-scored` — it doesn't
   match either wildcard, so it needs to be listed explicitly).
3. **`GET /transactions` requires `status`, no default "list everything".**
   The only consumer (mobile-app's Held Transactions screen) always wants
   `status=held`; an unfiltered "list all transactions ever" endpoint has
   no caller and would need pagination design that nothing currently
   needs — add it later if a real use case shows up.
4. **Approve/reject are unconditional once `held`** — no reason/comment
   field, no "who approved" field. There's no auth/identity system yet
   (per every other service's deferral of auth), so there's no real actor
   to attribute the action to; adding a free-text "approved_by" field
   nobody can verify would be theater, not an audit trail.

## New business rule (add to `docs/BUSINESS_RULES.md` in Task 3)

```markdown
- **SETTLEMENT-05** — `POST /transactions/{id}/approve` và
  `POST /transactions/{id}/reject` chỉ hợp lệ khi transaction đang ở status
  `held`; gọi trên bất kỳ status nào khác trả về `409`. Approve chuyển
  `held` → `pending_settlement` (transaction vào batch settle tiếp theo,
  giống hệt một transaction đã `pass` risk-scoring). Reject chuyển `held`
  → `rejected`, terminal — không bao giờ vào batch.
  **Vì sao:** giữ state machine đơn giản (không thêm status "approved"
  song song với "pending_settlement" vốn đã mang đúng ý nghĩa "sẵn sàng
  settle"); `rejected` phải terminal vì đây là quyết định con người ghi đè
  lên risk hold, không phải trạng thái tạm.
  **Ở đâu:** `services/settlement-engine/internal/settlement`
  (`TransactionRepository.Approve`/`.Reject`).
```

---

### Task 1: Migration — add `rejected` to the status CHECK constraint

**Files:**
- Create: `internal/db/migrations/000008_add_rejected_transaction_status.up.sql`
- Create: `internal/db/migrations/000008_add_rejected_transaction_status.down.sql`

- [ ] **Step 1: Write the migration**

`000008_add_rejected_transaction_status.up.sql`:

```sql
ALTER TABLE transactions DROP CONSTRAINT transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('pending_settlement', 'held', 'settled', 'rejected'));
```

`000008_add_rejected_transaction_status.down.sql`:

```sql
ALTER TABLE transactions DROP CONSTRAINT transactions_status_check;
ALTER TABLE transactions ADD CONSTRAINT transactions_status_check
    CHECK (status IN ('pending_settlement', 'held', 'settled'));
```

(Constraint name `transactions_status_check` is Postgres's default
auto-generated name for an unnamed column CHECK — confirm it with `\d
transactions` against a migrated test DB before relying on it; if
Postgres named it differently, adjust both files to match.)

- [ ] **Step 2: Verify against a real Postgres**

```bash
go test ./internal/db/... -run TestMigrate -v
```

Expected: passes, and a manual `\d transactions` on the test container (or
local `settlement-postgres`) shows `rejected` in the constraint.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/000008_add_rejected_transaction_status.*.sql
git commit -m "feat(settlement-engine): add rejected transaction status"
```

---

### Task 2: `Transaction` domain type + read methods

**Files:**
- Modify: `internal/settlement/transaction_repository.go`
- Test: `internal/settlement/transaction_repository_test.go`

**Interfaces:**
- Produces: `type Transaction struct` (id, amount, score, decision,
  status, triggered_rules, scored_at, account_ids),
  `TransactionRepository.Get(ctx, id) (*Transaction, error)` (returns
  `ErrTransactionNotFound`), `TransactionRepository.ListByStatus(ctx,
  status string) ([]Transaction, error)`. Task 5 (HTTP handlers) calls
  both directly.

- [ ] **Step 1: Write the failing tests**

Append to `internal/settlement/transaction_repository_test.go` (follow
the existing file's pattern for seeding a scored transaction via
`RecordScore` — reuse that helper if the file already has one):

```go
func TestGetTransaction(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))
	txID := uuid.New()
	accID := uuid.New()
	require.NoError(t, repo.RecordScore(ctx, []uuid.UUID{accID}, 1000, risk.RiskScore{
		TransactionID: txID, Score: 10, Decision: risk.DecisionPass,
	}))

	got, err := repo.Get(ctx, txID)

	require.NoError(t, err)
	assert.Equal(t, txID, got.ID)
	assert.Equal(t, []uuid.UUID{accID}, got.AccountIDs)
	assert.Equal(t, StatusPendingSettlement, got.Status)
}

func TestGetTransaction_NotFound(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))

	_, err := repo.Get(ctx, uuid.New())

	assert.ErrorIs(t, err, ErrTransactionNotFound)
}

func TestListByStatus_ReturnsOnlyMatchingStatus(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))
	heldID, passID := uuid.New(), uuid.New()
	accID := uuid.New()
	require.NoError(t, repo.RecordScore(ctx, []uuid.UUID{accID}, 1000, risk.RiskScore{
		TransactionID: heldID, Score: 90, Decision: risk.DecisionHold,
	}))
	require.NoError(t, repo.RecordScore(ctx, []uuid.UUID{accID}, 1000, risk.RiskScore{
		TransactionID: passID, Score: 0, Decision: risk.DecisionPass,
	}))

	held, err := repo.ListByStatus(ctx, StatusHeld)

	require.NoError(t, err)
	require.Len(t, held, 1)
	assert.Equal(t, heldID, held[0].ID)
}
```

- [ ] **Step 2: Run to verify it fails** (`ErrTransactionNotFound`, `Get`,
  `ListByStatus`, `Transaction` don't exist yet).

- [ ] **Step 3: Implement**

```go
// ErrTransactionNotFound is returned by Get/Approve/Reject when the
// transaction id doesn't exist.
var ErrTransactionNotFound = errors.New("transaction not found")

// Transaction is one risk-scored transaction as read back out of storage
// -- the read-side counterpart to the write-only RecordScore path above.
type Transaction struct {
	ID             uuid.UUID
	AccountIDs     []uuid.UUID
	Amount         int64
	Score          int
	Decision       string
	Status         string
	TriggeredRules []string
	ScoredAt       time.Time
}

// Get returns one transaction by id, including its account_ids joined
// from transaction_accounts.
func (r *TransactionRepository) Get(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	txn, err := r.scanOne(ctx, "WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	if txn == nil {
		return nil, ErrTransactionNotFound
	}
	return txn, nil
}

// ListByStatus returns every transaction currently at the given status,
// most recently scored first.
func (r *TransactionRepository) ListByStatus(ctx context.Context, status string) ([]Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, amount, score, decision, status, triggered_rules, scored_at
		FROM transactions WHERE status = $1 ORDER BY scored_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("list transactions by status: %w", err)
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		var triggeredRules pq.StringArray
		if err := rows.Scan(&t.ID, &t.Amount, &t.Score, &t.Decision, &t.Status, &triggeredRules, &t.ScoredAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		t.TriggeredRules = []string(triggeredRules)
		accountIDs, err := r.accountIDsFor(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		t.AccountIDs = accountIDs
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TransactionRepository) scanOne(ctx context.Context, where string, args ...any) (*Transaction, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, amount, score, decision, status, triggered_rules, scored_at
		FROM transactions `+where, args...)

	var t Transaction
	var triggeredRules pq.StringArray
	if err := row.Scan(&t.ID, &t.Amount, &t.Score, &t.Decision, &t.Status, &triggeredRules, &t.ScoredAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan transaction: %w", err)
	}
	t.TriggeredRules = []string(triggeredRules)

	accountIDs, err := r.accountIDsFor(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t.AccountIDs = accountIDs
	return &t, nil
}

func (r *TransactionRepository) accountIDsFor(ctx context.Context, transactionID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id FROM transaction_accounts WHERE transaction_id = $1
	`, transactionID)
	if err != nil {
		return nil, fmt.Errorf("list account_ids for transaction %s: %w", transactionID, err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan account_id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

Check which Postgres array-scanning helper the rest of the codebase
already uses for `triggered_rules TEXT[]` (`RecordScore` writes it as a Go
`[]string` via `pq.Array`/driver support — match whatever import is
already in use in this file/module rather than introducing a second
convention; `github.com/lib/pq`'s `pq.StringArray` is the illustrative
choice above but confirm against `go.mod`).

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/settlement/... -run 'TestGetTransaction|TestListByStatus' -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/settlement/transaction_repository.go internal/settlement/transaction_repository_test.go
git commit -m "feat(settlement-engine): Transaction read methods (Get, ListByStatus)"
```

---

### Task 3: Approve / Reject

**Files:**
- Modify: `internal/settlement/transaction_repository.go`
- Modify: `docs/BUSINESS_RULES.md` (add `SETTLEMENT-05`, see above)
- Test: `internal/settlement/transaction_repository_test.go`

**Interfaces:**
- Produces: `ErrTransactionNotHeld`,
  `TransactionRepository.Approve(ctx, id) (*Transaction, error)`,
  `TransactionRepository.Reject(ctx, id) (*Transaction, error)`. Task 5
  (HTTP handlers) calls both directly.
- Constant: `EventTransactionResolved = "transaction.resolved"`.

- [ ] **Step 1: Write the failing tests**

```go
func TestApprove_HeldToPendingSettlement(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))
	txID := seedHeldTransaction(t, repo)

	got, err := repo.Approve(ctx, txID)

	require.NoError(t, err)
	assert.Equal(t, StatusPendingSettlement, got.Status)
}

func TestReject_HeldToRejected(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))
	txID := seedHeldTransaction(t, repo)

	got, err := repo.Reject(ctx, txID)

	require.NoError(t, err)
	assert.Equal(t, StatusRejected, got.Status)
}

func TestApprove_NotHeld_ReturnsError(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))
	txID := uuid.New()
	accID := uuid.New()
	require.NoError(t, repo.RecordScore(ctx, []uuid.UUID{accID}, 1000, risk.RiskScore{
		TransactionID: txID, Score: 0, Decision: risk.DecisionPass, // -> pending_settlement, not held
	}))

	_, err := repo.Approve(ctx, txID)

	assert.ErrorIs(t, err, ErrTransactionNotHeld)
}

func TestApprove_NotFound_ReturnsError(t *testing.T) {
	repo := NewTransactionRepository(testutil.NewTestDB(t))

	_, err := repo.Approve(ctx, uuid.New())

	assert.ErrorIs(t, err, ErrTransactionNotFound)
}
```

(`seedHeldTransaction` is a small local test helper: calls `RecordScore`
with `Decision: risk.DecisionHold` and returns the transaction id — add it
once, reuse across this file's held-transaction tests.)

- [ ] **Step 2: Run to verify failure**, then implement.

```go
// StatusRejected is terminal, like StatusSettled: a human overrode a risk
// hold by rejecting it, never re-entered.
const StatusRejected = "rejected"

// ErrTransactionNotHeld is returned by Approve/Reject when the transaction
// isn't currently held.
var ErrTransactionNotHeld = errors.New("transaction is not held")

// EventTransactionResolved is published once per Approve/Reject call.
const EventTransactionResolved = "transaction.resolved"

// ResolvedPayload is the JSON body written to outbox_events for a
// transaction.resolved event.
type ResolvedPayload struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Resolution    string    `json:"resolution"` // "approved" | "rejected"
	ResolvedAt    time.Time `json:"resolved_at"`
}

// Approve moves a held transaction back into the normal settlement flow.
// See SETTLEMENT-05 in docs/BUSINESS_RULES.md.
func (r *TransactionRepository) Approve(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	return r.resolve(ctx, id, StatusPendingSettlement, "approved")
}

// Reject moves a held transaction to the terminal rejected status. See
// SETTLEMENT-05 in docs/BUSINESS_RULES.md.
func (r *TransactionRepository) Reject(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	return r.resolve(ctx, id, StatusRejected, "rejected")
}

func (r *TransactionRepository) resolve(ctx context.Context, id uuid.UUID, newStatus, resolution string) (*Transaction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM transactions WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("lock transaction: %w", err)
	}
	if currentStatus != StatusHeld {
		return nil, ErrTransactionNotHeld
	}

	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET status = $1 WHERE id = $2`, newStatus, id); err != nil {
		return nil, fmt.Errorf("update transaction status: %w", err)
	}

	resolvedAt := time.Now().UTC()
	body, err := json.Marshal(ResolvedPayload{TransactionID: id, Resolution: resolution, ResolvedAt: resolvedAt})
	if err != nil {
		return nil, fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, event_type, subject, payload) VALUES ($1, $2, $3, $4)
	`, uuid.New(), EventTransactionResolved, EventTransactionResolved, body); err != nil {
		return nil, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return r.Get(ctx, id)
}
```

- [ ] **Step 3: Run to verify pass**, add the `SETTLEMENT-05` block to
  `docs/BUSINESS_RULES.md` (exact text above, in the `## settlement-engine`
  section after `SETTLEMENT-04`).

- [ ] **Step 4: Commit**

```bash
git add internal/settlement/transaction_repository.go internal/settlement/transaction_repository_test.go docs/BUSINESS_RULES.md
git commit -m "feat(settlement-engine): Approve/Reject held transactions, publish transaction.resolved"
```

---

### Task 4: Settlement read methods

**Files:**
- Modify: `internal/settlement/settlement_repository.go`
- Test: `internal/settlement/settlement_repository_test.go`

**Interfaces:**
- Produces: `SettlementRepository.Get(ctx, id) (*Settlement, error)`
  (returns `ErrSettlementNotFound`), `SettlementRepository.List(ctx)
  ([]Settlement, error)` (most recent first). Task 5 calls both.

- [ ] **Step 1: Write the failing tests** (seed via `RunBatch` after
  recording a pass-decision transaction, same pattern
  `settlement_repository_test.go` already uses for `RunBatch` tests).

```go
func TestGetSettlement(t *testing.T) {
	settlements := NewSettlementRepository(testutil.NewTestDB(t))
	seeded := runBatchWithOnePassingTransaction(t, settlements) // existing test helper or inline setup

	got, err := settlements.Get(ctx, seeded.ID)

	require.NoError(t, err)
	assert.Equal(t, seeded.TransactionCount, got.TransactionCount)
}

func TestGetSettlement_NotFound(t *testing.T) {
	settlements := NewSettlementRepository(testutil.NewTestDB(t))

	_, err := settlements.Get(ctx, uuid.New())

	assert.ErrorIs(t, err, ErrSettlementNotFound)
}

func TestListSettlements_MostRecentFirst(t *testing.T) {
	settlements := NewSettlementRepository(testutil.NewTestDB(t))
	first := runBatchWithOnePassingTransaction(t, settlements)
	second := runBatchWithOnePassingTransaction(t, settlements)

	list, err := settlements.List(ctx)

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, second.ID, list[0].ID)
	assert.Equal(t, first.ID, list[1].ID)
}
```

- [ ] **Step 2: Implement**

```go
var ErrSettlementNotFound = errors.New("settlement not found")

func (r *SettlementRepository) Get(ctx context.Context, id uuid.UUID) (*Settlement, error) {
	var s Settlement
	err := r.db.QueryRowContext(ctx, `
		SELECT id, transaction_count, total_amount, created_at FROM settlements WHERE id = $1
	`, id).Scan(&s.ID, &s.TransactionCount, &s.TotalAmount, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSettlementNotFound
		}
		return nil, fmt.Errorf("get settlement: %w", err)
	}
	ids, err := r.transactionIDsFor(ctx, id)
	if err != nil {
		return nil, err
	}
	s.TransactionIDs = ids
	return &s, nil
}

func (r *SettlementRepository) List(ctx context.Context) ([]Settlement, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, transaction_count, total_amount, created_at FROM settlements ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list settlements: %w", err)
	}
	defer rows.Close()

	var out []Settlement
	for rows.Next() {
		var s Settlement
		if err := rows.Scan(&s.ID, &s.TransactionCount, &s.TotalAmount, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan settlement: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SettlementRepository) transactionIDsFor(ctx context.Context, settlementID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT transaction_id FROM settlement_transactions WHERE settlement_id = $1
	`, settlementID)
	if err != nil {
		return nil, fmt.Errorf("list transaction_ids for settlement %s: %w", settlementID, err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan transaction_id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
```

(`List`'s per-row `transactionIDsFor` N+1 is acceptable at MVP scale —
settlements batch runs are infrequent (`SETTLEMENT_BATCH_INTERVAL_SECONDS`,
default 60s) and this endpoint has one caller, a mobile app screen with no
pagination yet. Revisit with a join if/when that stops being true.)

- [ ] **Step 3: Run to verify pass, commit**

```bash
git add internal/settlement/settlement_repository.go internal/settlement/settlement_repository_test.go
git commit -m "feat(settlement-engine): Settlement read methods (Get, List)"
```

---

### Task 5: HTTP handlers + router

**Files:**
- Create: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `settlement.TransactionRepository` and
  `settlement.SettlementRepository` (Tasks 2-4).
- Produces: `api.NewRouter(transactions, settlements)` — signature change
  from today's `api.NewRouter()`; Task 6 updates the one call site in
  `main.go`.

- [ ] **Step 1: Write failing `httptest`-based handler tests** — follow
  accounts-service's `internal/api/handlers_account_test.go` pattern
  exactly (spin up a `httptest.Server` wrapping the real router against a
  real testcontainers Postgres, no mocks): seed a held transaction via the
  repository directly, then assert `GET /transactions?status=held` returns
  it, `POST /transactions/{id}/approve` returns `200` and flips it,
  `POST /transactions/{id}/approve` called *again* on the same id now
  returns `409`, `POST /transactions/{id}/approve` on a random id returns
  `404`. Mirror for reject and for the two settlement endpoints.

- [ ] **Step 2: Implement `internal/api/handlers.go`**

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

type Handlers struct {
	transactions *settlement.TransactionRepository
	settlements  *settlement.SettlementRepository
}

func NewHandlers(transactions *settlement.TransactionRepository, settlements *settlement.SettlementRepository) *Handlers {
	return &Handlers{transactions: transactions, settlements: settlements}
}

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)

	r.Get("/transactions", h.ListTransactions)
	r.Get("/transactions/{id}", h.GetTransaction)
	r.Post("/transactions/{id}/approve", h.ApproveTransaction)
	r.Post("/transactions/{id}/reject", h.RejectTransaction)

	r.Get("/settlements", h.ListSettlements)
	r.Get("/settlements/{id}", h.GetSettlement)

	return r
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type transactionResponse struct {
	ID             string   `json:"id"`
	AccountIDs     []string `json:"account_ids"`
	Amount         int64    `json:"amount"`
	Score          int      `json:"score"`
	Decision       string   `json:"decision"`
	Status         string   `json:"status"`
	TriggeredRules []string `json:"triggered_rules"`
	ScoredAt       string   `json:"scored_at"`
}

func toTransactionResponse(t settlement.Transaction) transactionResponse {
	accountIDs := make([]string, len(t.AccountIDs))
	for i, id := range t.AccountIDs {
		accountIDs[i] = id.String()
	}
	return transactionResponse{
		ID: t.ID.String(), AccountIDs: accountIDs, Amount: t.Amount, Score: t.Score,
		Decision: t.Decision, Status: t.Status, TriggeredRules: t.TriggeredRules,
		ScoredAt: t.ScoredAt.Format(timeFormat),
	}
}

const timeFormat = "2006-01-02T15:04:05.000Z07:00"

func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		writeError(w, http.StatusBadRequest, "status query param is required")
		return
	}
	txs, err := h.transactions.ListByStatus(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]transactionResponse, len(txs))
	for i, t := range txs {
		resp[i] = toTransactionResponse(t)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
	t, err := h.transactions.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, settlement.ErrTransactionNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toTransactionResponse(*t))
}

func (h *Handlers) ApproveTransaction(w http.ResponseWriter, r *http.Request) {
	h.resolveTransaction(w, r, h.transactions.Approve)
}

func (h *Handlers) RejectTransaction(w http.ResponseWriter, r *http.Request) {
	h.resolveTransaction(w, r, h.transactions.Reject)
}

func (h *Handlers) resolveTransaction(w http.ResponseWriter, r *http.Request, resolve func(context.Context, uuid.UUID) (*settlement.Transaction, error)) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
	t, err := resolve(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, settlement.ErrTransactionNotFound):
			writeError(w, http.StatusNotFound, "transaction not found")
		case errors.Is(err, settlement.ErrTransactionNotHeld):
			writeError(w, http.StatusConflict, "transaction is not held")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, toTransactionResponse(*t))
}

type settlementResponse struct {
	ID               string   `json:"id"`
	TransactionIDs   []string `json:"transaction_ids"`
	TransactionCount int      `json:"transaction_count"`
	TotalAmount      int64    `json:"total_amount"`
	CreatedAt        string   `json:"created_at"`
}

func toSettlementResponse(s settlement.Settlement) settlementResponse {
	ids := make([]string, len(s.TransactionIDs))
	for i, id := range s.TransactionIDs {
		ids[i] = id.String()
	}
	return settlementResponse{
		ID: s.ID.String(), TransactionIDs: ids, TransactionCount: s.TransactionCount,
		TotalAmount: s.TotalAmount, CreatedAt: s.CreatedAt.Format(timeFormat),
	}
}

func (h *Handlers) ListSettlements(w http.ResponseWriter, r *http.Request) {
	list, err := h.settlements.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	resp := make([]settlementResponse, len(list))
	for i, s := range list {
		resp[i] = toSettlementResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) GetSettlement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid settlement id")
		return
	}
	s, err := h.settlements.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, settlement.ErrSettlementNotFound) {
			writeError(w, http.StatusNotFound, "settlement not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toSettlementResponse(*s))
}
```

(add the missing `"context"` import; delete the old
`internal/api/router.go`'s bare `NewRouter()`/`health` — superseded by
`NewHandlers`/`NewRouter(h)` above, same restructuring
accounts-service already went through.)

- [ ] **Step 3: Run full test suite, verify pass**

```bash
go test ./... -v
```

- [ ] **Step 4: Update README's API section** (mirror
  accounts-service/ledger-service's `## API` bullet-list format) and
  commit

```bash
git add internal/api/ README.md
git commit -m "feat(settlement-engine): HTTP API for transaction/settlement read + approve/reject"
```

---

### Task 6: Wire `main.go`

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update the two call sites**

Change `router := api.NewRouter()` to:

```go
handlers := api.NewHandlers(transactions, settlements)
router := api.NewRouter(handlers)
```

(`transactions`/`settlements` are already constructed earlier in `main`.)

Add `"transaction.resolved"` to the `SettlementEventsStream` subject list:

```go
if err := broker.EnsureStream(ctx, js, jetstream.StreamConfig{
    Name:     broker.SettlementEventsStream,
    Subjects: []string{"settlement.>", "transaction.risk-scored", "transaction.resolved"},
    Storage:  jetstream.FileStorage,
}); err != nil {
    log.Fatalf("ensure settlement events stream: %v", err)
}
```

- [ ] **Step 2: Manual smoke test**

```bash
docker compose -f infra/docker/docker-compose.yml up -d settlement-postgres ledger-postgres nats
go run ./cmd/server
```

In another terminal, run a transaction through ledger-service that
triggers a hold (see settlement-engine README's risk-scoring rules for
what triggers one), then:

```bash
curl "http://localhost:8082/transactions?status=held"
curl -X POST "http://localhost:8082/transactions/<id>/approve"
curl "http://localhost:8082/settlements"
```

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(settlement-engine): wire new HTTP handlers and transaction.resolved subject"
```
