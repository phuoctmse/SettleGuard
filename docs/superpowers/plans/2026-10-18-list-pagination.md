# List pagination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Phân trang keyset cho `GET /transactions?status=…` và `GET /settlements` của settlement-engine, và tab Held/Settlements của `mobile-app` cuộn tới đâu tải tới đó — theo `docs/superpowers/specs/2026-10-18-list-pagination-design.md`.

**Architecture:** Repository nhận `limit` và một `settlement.Cursor` (timestamp + id của dòng cuối trang trước), lấy `limit + 1` dòng theo `ORDER BY <ts> DESC, id DESC` với điều kiện `(<ts>, id) < ($t, $id)`, và trả cursor cho trang sau nếu có dòng dư. Tầng HTTP lo mặc định/giới hạn `limit`, mã hoá cursor thành chuỗi base64url opaque và bọc kết quả trong `{"items": [...], "next_cursor": …}`. App chuyển hai màn hình sang `useInfiniteQuery` và gọi trang sau ở `onEndReached`.

**Tech Stack:** Go 1.26, chi, `database/sql` + pgx, golang-migrate, testcontainers-go; Python pytest + httpx; Expo/React Native, `@tanstack/react-query` v5, `@testing-library/react-native` 14.

## Global Constraints

- Branch: `step/list-pagination` (đã tạo từ `origin/main`, spec đã commit). Chạy `git branch --show-current` ngay trước **mỗi** commit.
- Commit message **không** có dòng `Co-Authored-By` hay "Generated with Claude".
- Response list: `{"items": [...], "next_cursor": "<opaque>" | null}`. `items` luôn là mảng (không bao giờ `null`).
- Thứ tự: `/transactions` theo `scored_at DESC, id DESC`; `/settlements` theo `created_at DESC, id DESC`.
- `limit`: không có → 50; > 200 → 200; < 1 hoặc không phải số nguyên → `400 {"error":"invalid limit"}`.
- `cursor`: base64url **không padding** của JSON `{"t":"<RFC3339Nano>","id":"<uuid>"}`. Không giải mã được / JSON sai / `t` hoặc `id` không hợp lệ → `400 {"error":"invalid cursor"}`.
- Có trang sau hay không: lấy `limit + 1` dòng; không dùng `COUNT(*)`.
- `/transactions` vẫn đòi `status` (`400 {"error":"status query param is required"}` như hiện nay).
- App: không gửi `limit`; giữ nguyên mọi testID, text hiển thị, style, điều hướng; thêm testID `held-list`, `settlements-list` trên `FlatList`; footer `"Loading more…"` / `"Failed to load more."`.
- Go test chạy `go test -count=1 -p 1 ./...` (Docker trên Windows rớt ngẫu nhiên khi song song).
- Không đổi gateway, notification-service, các tab Accounts/Alerts.

## File Structure

| File | Trách nhiệm |
|---|---|
| `services/settlement-engine/internal/db/migrations/000010_add_list_pagination_indexes.{up,down}.sql` (tạo) | Index cho hai thứ tự |
| `services/settlement-engine/internal/settlement/cursor.go` (tạo) | Kiểu `Cursor` |
| `services/settlement-engine/internal/settlement/transaction_repository.go` (sửa) | `ListByStatusPage` thay `ListByStatus` |
| `services/settlement-engine/internal/settlement/settlement_repository.go` (sửa) | `ListPage` thay `List` |
| `services/settlement-engine/internal/api/pagination.go` (tạo) | `parseLimit`, `parseCursor`, `encodeCursor`, `pageResponse` |
| `services/settlement-engine/internal/api/handlers.go` (sửa) | Hai handler list dùng phân trang |
| `services/settlement-engine/README.md` (sửa) | Mô tả API |
| `docs/openapi/settlement-engine.yaml` (sửa) | Tham số, schema envelope, 400 |
| `tests/api/clients/settlement.py`, `tests/api/test_settlement_contract.py`, `tests/security/test_fraud_bypass.py` (sửa) | Bên tiêu thụ trong `tests/` |
| `mobile-app/src/api/types.ts`, `settlement.ts` (sửa) | `Page<T>`, cursor |
| `mobile-app/src/components/LoadMoreFooter.tsx` (+ test) (tạo) | Footer "Loading more…" / "Failed to load more." |
| `mobile-app/src/screens/HeldTransactionsScreen.tsx`, `SettlementsScreen.tsx` (+ test) (sửa) | `useInfiniteQuery` |

---

### Task 1: Index và repository keyset

**Files:**
- Create: `services/settlement-engine/internal/db/migrations/000010_add_list_pagination_indexes.up.sql`, `.down.sql`
- Create: `services/settlement-engine/internal/settlement/cursor.go`
- Modify: `services/settlement-engine/internal/settlement/transaction_repository.go`, `settlement_repository.go`
- Test: `services/settlement-engine/internal/settlement/transaction_repository_test.go`, `settlement_repository_test.go`

**Interfaces:**
- Produces:
  - `type Cursor struct { At time.Time; ID uuid.UUID }` (package `settlement`)
  - `func (r *TransactionRepository) ListByStatusPage(ctx context.Context, status string, limit int, after *Cursor) ([]Transaction, *Cursor, error)`
  - `func (r *SettlementRepository) ListPage(ctx context.Context, limit int, after *Cursor) ([]Settlement, *Cursor, error)`
  - Ngữ nghĩa: trả tối đa `limit` phần tử, bắt đầu **sau** `after` (nil = từ đầu); cursor trả về là của phần tử cuối trang, `nil` nếu không còn trang sau. Repository không tự áp mặc định cho `limit` (caller phải truyền ≥ 1).
- `ListByStatus` và `List` cũ **giữ nguyên** ở task này (handler còn dùng); Task 2 xoá chúng.

- [ ] **Step 1: Migration**

`000010_add_list_pagination_indexes.up.sql`:

```sql
CREATE INDEX idx_transactions_status_scored ON transactions (status, scored_at DESC, id DESC);
CREATE INDEX idx_settlements_created ON settlements (created_at DESC, id DESC);
```

`000010_add_list_pagination_indexes.down.sql`:

```sql
DROP INDEX IF EXISTS idx_settlements_created;
DROP INDEX IF EXISTS idx_transactions_status_scored;
```

- [ ] **Step 2: Kiểu `Cursor`**

`internal/settlement/cursor.go`:

```go
package settlement

import (
	"time"

	"github.com/google/uuid"
)

// Cursor is a position in a keyset-paginated list: the sort timestamp and id
// of the last row already returned. A page continues strictly after it, so
// rows approved or rejected between two requests never shift later pages.
type Cursor struct {
	At time.Time
	ID uuid.UUID
}
```

- [ ] **Step 3: Viết test thất bại cho transactions**

Thêm vào cuối `internal/settlement/transaction_repository_test.go` (package `settlement_test`; thêm `"database/sql"` và `"time"` vào import nếu chưa có):

```go
func insertHeldAt(t *testing.T, db *sql.DB, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO transactions (id, amount, score, decision, status, triggered_rules, scored_at)
		VALUES ($1, 1000, 40, 'hold', 'held', '{velocity_limit}', $2)
	`, id, at)
	require.NoError(t, err)
}

func collectHeld(t *testing.T, repo *settlement.TransactionRepository, limit int) ([]uuid.UUID, int) {
	t.Helper()
	var ids []uuid.UUID
	var after *settlement.Cursor
	pages := 0
	for {
		page, next, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, limit, after)
		require.NoError(t, err)
		require.LessOrEqual(t, len(page), limit)
		pages++
		for _, tx := range page {
			ids = append(ids, tx.ID)
		}
		if next == nil {
			return ids, pages
		}
		after = next
		require.Less(t, pages, 100, "pagination did not terminate")
	}
}

func TestListByStatusPage_WalksEveryRowOnceNewestFirst(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	var want []uuid.UUID // newest first
	for i := 4; i >= 0; i-- {
		id := uuid.New()
		insertHeldAt(t, db, id, base.Add(time.Duration(i)*time.Second))
		want = append(want, id)
	}
	require.NoError(t, repo.RecordScore(context.Background(), []uuid.UUID{uuid.New()}, 1_000, passScore(uuid.New())))

	got, pages := collectHeld(t, repo, 2)

	assert.Equal(t, want, got)
	assert.Equal(t, 3, pages)
}

func TestListByStatusPage_BreaksTimestampTiesByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	at := time.Date(2026, 10, 18, 12, 0, 0, 123456000, time.UTC)

	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		insertHeldAt(t, db, id, at)
	}
	// Postgres orders uuid bytewise, which matches the canonical string order.
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() > ids[j].String() })

	got, pages := collectHeld(t, repo, 1)

	assert.Equal(t, ids, got)
	assert.Equal(t, 3, pages)
}

func TestListByStatusPage_SinglePageHasNoCursor(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewTransactionRepository(db)
	insertHeldAt(t, db, uuid.New(), time.Now())

	page, next, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, 50, nil)

	require.NoError(t, err)
	assert.Len(t, page, 1)
	assert.Nil(t, next)
}
```

(Thêm `"sort"` vào import.)

- [ ] **Step 4: Viết test thất bại cho settlements**

Thêm vào cuối `internal/settlement/settlement_repository_test.go` (thêm `"sort"`, `"time"` vào import nếu chưa có):

```go
func insertSettlementAt(t *testing.T, db *sql.DB, id uuid.UUID, at time.Time) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO settlements (id, transaction_count, total_amount, created_at) VALUES ($1, 1, 100, $2)
	`, id, at)
	require.NoError(t, err)
}

func TestListSettlementsPage_WalksEveryRowOnceNewestFirstWithTies(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	newest := uuid.New()
	insertSettlementAt(t, db, newest, base.Add(time.Minute))
	tied := []uuid.UUID{uuid.New(), uuid.New()}
	for _, id := range tied {
		insertSettlementAt(t, db, id, base)
	}
	sort.Slice(tied, func(i, j int) bool { return tied[i].String() > tied[j].String() })
	want := append([]uuid.UUID{newest}, tied...)

	var got []uuid.UUID
	var after *settlement.Cursor
	for pages := 0; ; pages++ {
		require.Less(t, pages, 10, "pagination did not terminate")
		page, next, err := repo.ListPage(context.Background(), 1, after)
		require.NoError(t, err)
		require.Len(t, page, 1)
		got = append(got, page[0].ID)
		if next == nil {
			break
		}
		after = next
	}

	assert.Equal(t, want, got)
}

func TestListSettlementsPage_IncludesTransactionIDs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := settlement.NewSettlementRepository(db)
	tx1, tx2 := uuid.New(), uuid.New()
	insertTransactionWithStatus(t, db, tx1, 1_000, settlement.StatusPendingSettlement)
	insertTransactionWithStatus(t, db, tx2, 2_000, settlement.StatusPendingSettlement)
	_, err := repo.RunBatch(context.Background())
	require.NoError(t, err)

	page, next, err := repo.ListPage(context.Background(), 50, nil)

	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Nil(t, next)
	assert.ElementsMatch(t, []uuid.UUID{tx1, tx2}, page[0].TransactionIDs)
}
```

- [ ] **Step 5: Chạy để xác nhận fail**

Run: `cd services/settlement-engine && go test -count=1 -p 1 ./internal/settlement/ -run 'Page'`
Expected: lỗi biên dịch `repo.ListByStatusPage undefined` / `repo.ListPage undefined`.

- [ ] **Step 6: Cài đặt `ListByStatusPage`**

Thêm vào `transaction_repository.go` (ngay sau `ListByStatus`):

```go
// ListByStatusPage returns up to limit transactions at status, most recently
// scored first with ties broken by id, starting strictly after `after` (nil
// starts from the top). next is the cursor of the page's last row, or nil
// when nothing follows. It fetches limit+1 rows to know that without a COUNT.
func (r *TransactionRepository) ListByStatusPage(ctx context.Context, status string, limit int, after *Cursor) ([]Transaction, *Cursor, error) {
	const query = `SELECT id, amount, score, decision, status, triggered_rules, scored_at FROM transactions`
	var (
		rows *sql.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.QueryContext(ctx, query+`
			WHERE status = $1 ORDER BY scored_at DESC, id DESC LIMIT $2`, status, limit+1)
	} else {
		rows, err = r.db.QueryContext(ctx, query+`
			WHERE status = $1 AND (scored_at, id) < ($2, $3)
			ORDER BY scored_at DESC, id DESC LIMIT $4`, status, after.At, after.ID, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("list transactions by status: %w", err)
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan transaction: %w", err)
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	// Close before the per-transaction queries so each one can reuse this
	// connection instead of opening another while rows is held.
	rows.Close()

	var next *Cursor
	if len(out) > limit {
		out = out[:limit]
		last := out[limit-1]
		next = &Cursor{At: last.ScoredAt, ID: last.ID}
	}
	for i := range out {
		accountIDs, err := r.accountIDsFor(ctx, out[i].ID)
		if err != nil {
			return nil, nil, err
		}
		out[i].AccountIDs = accountIDs
	}
	return out, next, nil
}
```

- [ ] **Step 7: Cài đặt `ListPage`**

Thêm vào `settlement_repository.go` (ngay sau `List`; thêm `"database/sql"` vào import nếu chưa có):

```go
// ListPage returns up to limit settlements, most recently created first with
// ties broken by id, starting strictly after `after` (nil starts from the
// top). next is the cursor of the page's last row, or nil when nothing follows.
func (r *SettlementRepository) ListPage(ctx context.Context, limit int, after *Cursor) ([]Settlement, *Cursor, error) {
	const query = `SELECT id, transaction_count, total_amount, created_at FROM settlements`
	var (
		rows *sql.Rows
		err  error
	)
	if after == nil {
		rows, err = r.db.QueryContext(ctx, query+`
			ORDER BY created_at DESC, id DESC LIMIT $1`, limit+1)
	} else {
		rows, err = r.db.QueryContext(ctx, query+`
			WHERE (created_at, id) < ($1, $2)
			ORDER BY created_at DESC, id DESC LIMIT $3`, after.At, after.ID, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("list settlements: %w", err)
	}
	defer rows.Close()

	var out []Settlement
	for rows.Next() {
		var s Settlement
		if err := rows.Scan(&s.ID, &s.TransactionCount, &s.TotalAmount, &s.CreatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan settlement: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows.Close()

	var next *Cursor
	if len(out) > limit {
		out = out[:limit]
		last := out[limit-1]
		next = &Cursor{At: last.CreatedAt, ID: last.ID}
	}
	for i := range out {
		ids, err := r.transactionIDsFor(ctx, out[i].ID)
		if err != nil {
			return nil, nil, err
		}
		out[i].TransactionIDs = ids
	}
	return out, next, nil
}
```

- [ ] **Step 8: Chạy lại**

Run: `cd services/settlement-engine && go build ./... && go vet ./... && go test -count=1 -p 1 ./internal/settlement/ && golangci-lint run ./...`
Expected: tất cả PASS (test cũ lẫn mới), lint sạch. Migration 000010 được `testutil.NewTestDB` áp dụng tự động; nếu migration lỗi, test sẽ fail ở bước tạo DB.

- [ ] **Step 9: Commit**

```bash
git branch --show-current   # step/list-pagination
git add services/settlement-engine/internal/db/migrations/000010_add_list_pagination_indexes.up.sql services/settlement-engine/internal/db/migrations/000010_add_list_pagination_indexes.down.sql services/settlement-engine/internal/settlement/cursor.go services/settlement-engine/internal/settlement/transaction_repository.go services/settlement-engine/internal/settlement/settlement_repository.go services/settlement-engine/internal/settlement/transaction_repository_test.go services/settlement-engine/internal/settlement/settlement_repository_test.go
git commit -m "feat(settlement-engine): keyset-paginated list queries and their indexes"
```

---

### Task 2: Handler — `limit`, `cursor`, envelope

**Files:**
- Create: `services/settlement-engine/internal/api/pagination.go`
- Modify: `services/settlement-engine/internal/api/handlers.go`, `services/settlement-engine/internal/settlement/transaction_repository.go`, `settlement_repository.go` (xoá hàm cũ), `services/settlement-engine/README.md`
- Test: `services/settlement-engine/internal/api/handlers_test.go`, `internal/settlement/transaction_repository_test.go`, `settlement_repository_test.go`

**Interfaces:**
- Consumes: `settlement.Cursor`, `ListByStatusPage`, `ListPage` (Task 1).
- Produces: HTTP contract trong Global Constraints. Tên nội bộ: `parseLimit(*http.Request) (int, error)`, `parseCursor(*http.Request) (*settlement.Cursor, error)`, `encodeCursor(*settlement.Cursor) *string`, `type pageResponse struct { Items any; NextCursor *string }`.

- [ ] **Step 1: Viết test thất bại**

Trong `internal/api/handlers_test.go` thêm helper (thêm `"encoding/base64"`, `"fmt"`, `"io"` vào import nếu chưa có):

```go
type pageBody struct {
	Items      []map[string]any `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

func getPage(t *testing.T, url string) pageBody {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body pageBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotNil(t, body.Items, "items must be an array, never null")
	return body
}
```

Sửa hai test hiện có cho đúng envelope:

```go
func TestListTransactions_ReturnsHeldOnly(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	heldID := seedHeldTransaction(t, transactions)

	body := getPage(t, server.URL+"/transactions?status=held")

	require.Len(t, body.Items, 1)
	assert.Equal(t, heldID.String(), body.Items[0]["id"])
	assert.Equal(t, "held", body.Items[0]["status"])
	assert.Nil(t, body.NextCursor)
}
```

```go
func TestListSettlements_ReturnsFinalizedBatches(t *testing.T) {
	server, transactions, settlements := newTestServer(t)
	txID := uuid.New()
	acc := uuid.New()
	require.NoError(t, transactions.RecordScore(context.Background(), []uuid.UUID{acc}, 5_000, risk.RiskScore{
		TransactionID: txID, Score: 0, Decision: risk.DecisionPass,
	}))
	seeded, err := settlements.RunBatch(context.Background())
	require.NoError(t, err)
	require.NotNil(t, seeded)

	body := getPage(t, server.URL+"/settlements")

	require.Len(t, body.Items, 1)
	assert.Equal(t, seeded.ID.String(), body.Items[0]["id"])
	assert.Nil(t, body.NextCursor)
}
```

Thêm test mới:

```go
func TestListTransactions_PagesWithCursor(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	seeded := map[string]bool{}
	for range 3 {
		seeded[seedHeldTransaction(t, transactions).String()] = true
	}

	first := getPage(t, server.URL+"/transactions?status=held&limit=2")
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor)

	second := getPage(t, server.URL+"/transactions?status=held&limit=2&cursor="+*first.NextCursor)
	require.Len(t, second.Items, 1)
	assert.Nil(t, second.NextCursor)

	got := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		got[item["id"].(string)] = true
	}
	assert.Equal(t, seeded, got)
}

func TestListTransactions_DefaultAndMaximumLimit(t *testing.T) {
	server, transactions, _ := newTestServer(t)
	for range 201 {
		seedHeldTransaction(t, transactions)
	}

	assert.Len(t, getPage(t, server.URL+"/transactions?status=held").Items, 50)
	assert.Len(t, getPage(t, server.URL+"/transactions?status=held&limit=500").Items, 200)
}

func TestListSettlements_PagesWithCursor(t *testing.T) {
	server, transactions, settlements := newTestServer(t)
	var batches []uuid.UUID
	for range 2 {
		require.NoError(t, transactions.RecordScore(context.Background(), []uuid.UUID{uuid.New()}, 5_000, risk.RiskScore{
			TransactionID: uuid.New(), Score: 0, Decision: risk.DecisionPass,
		}))
		s, err := settlements.RunBatch(context.Background())
		require.NoError(t, err)
		require.NotNil(t, s)
		batches = append(batches, s.ID)
	}

	first := getPage(t, server.URL+"/settlements?limit=1")
	require.Len(t, first.Items, 1)
	require.NotNil(t, first.NextCursor)
	second := getPage(t, server.URL+"/settlements?limit=1&cursor="+*first.NextCursor)
	require.Len(t, second.Items, 1)
	assert.Nil(t, second.NextCursor)

	assert.Equal(t, batches[1].String(), first.Items[0]["id"]) // newest first
	assert.Equal(t, batches[0].String(), second.Items[0]["id"])
}

func TestListEndpoints_RejectBadPaging(t *testing.T) {
	server, _, _ := newTestServer(t)
	notJSON := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	badID := base64.RawURLEncoding.EncodeToString([]byte(`{"t":"2026-10-18T12:00:00Z","id":"nope"}`))
	badTime := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"t":"yesterday","id":"%s"}`, uuid.New())))

	cases := []struct{ query, message string }{
		{"limit=0", "invalid limit"},
		{"limit=-1", "invalid limit"},
		{"limit=abc", "invalid limit"},
		{"cursor=%25%25%25", "invalid cursor"},
		{"cursor=" + notJSON, "invalid cursor"},
		{"cursor=" + badID, "invalid cursor"},
		{"cursor=" + badTime, "invalid cursor"},
	}
	for _, path := range []string{"/transactions?status=held&", "/settlements?"} {
		for _, c := range cases {
			t.Run(path+c.query, func(t *testing.T) {
				resp, err := http.Get(server.URL + path + c.query)
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				assert.JSONEq(t, fmt.Sprintf(`{"error":%q}`, c.message), string(body))
			})
		}
	}
}
```

- [ ] **Step 2: Chạy để xác nhận fail**

Run: `cd services/settlement-engine && go test -count=1 -p 1 ./internal/api/`
Expected: FAIL — decode envelope lỗi (`cannot unmarshal array into Go value of type api_test.pageBody`) và các case 400 nhận 200.

- [ ] **Step 3: Viết `internal/api/pagination.go`**

```go
package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/settlement-engine/internal/settlement"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

var (
	errInvalidLimit  = errors.New("invalid limit")
	errInvalidCursor = errors.New("invalid cursor")
)

// pageResponse is the body of every list endpoint. Items is always a JSON
// array; NextCursor is null on the last page.
type pageResponse struct {
	Items      any     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// cursorWire is the JSON inside the opaque base64url cursor. The timestamp
// keeps full precision -- the API's display format stops at milliseconds,
// which would skip or repeat rows scored within the same millisecond.
type cursorWire struct {
	T  string `json:"t"`
	ID string `json:"id"`
}

// parseLimit reads ?limit=: absent means the default, above the maximum is
// clamped, and anything below 1 or non-numeric is rejected (a lower bound the
// notification-service lacked -- SG-PT-07).
func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultPageLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, errInvalidLimit
	}
	return min(n, maxPageLimit), nil
}

// parseCursor reads ?cursor=; absent means the first page.
func parseCursor(r *http.Request) (*settlement.Cursor, error) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, errInvalidCursor
	}
	var wire cursorWire
	if err := json.Unmarshal(decoded, &wire); err != nil {
		return nil, errInvalidCursor
	}
	at, err := time.Parse(time.RFC3339Nano, wire.T)
	if err != nil {
		return nil, errInvalidCursor
	}
	id, err := uuid.Parse(wire.ID)
	if err != nil {
		return nil, errInvalidCursor
	}
	return &settlement.Cursor{At: at, ID: id}, nil
}

// encodeCursor turns the repository's next-page position into the opaque
// string clients send back; nil (no next page) stays nil, which encodes as null.
func encodeCursor(c *settlement.Cursor) *string {
	if c == nil {
		return nil
	}
	// Marshalling two strings cannot fail.
	b, _ := json.Marshal(cursorWire{T: c.At.UTC().Format(time.RFC3339Nano), ID: c.ID.String()})
	s := base64.RawURLEncoding.EncodeToString(b)
	return &s
}
```

- [ ] **Step 4: Sửa hai handler trong `handlers.go`**

```go
func (h *Handlers) ListTransactions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		writeError(w, http.StatusBadRequest, "status query param is required")
		return
	}
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	after, err := parseCursor(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	txs, next, err := h.transactions.ListByStatusPage(r.Context(), status, limit, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]transactionResponse, len(txs))
	for i, t := range txs {
		resp[i] = toTransactionResponse(t)
	}
	writeJSON(w, http.StatusOK, pageResponse{Items: resp, NextCursor: encodeCursor(next)})
}
```

```go
func (h *Handlers) ListSettlements(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	after, err := parseCursor(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	list, next, err := h.settlements.ListPage(r.Context(), limit, after)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]settlementResponse, len(list))
	for i, s := range list {
		resp[i] = toSettlementResponse(s)
	}
	writeJSON(w, http.StatusOK, pageResponse{Items: resp, NextCursor: encodeCursor(next)})
}
```

Giữ nguyên comment phía trên `ListTransactions` nếu có, sửa câu nào nói "list everything ... would need pagination nothing currently needs" cho đúng hiện trạng: `// Paginated: see pagination.go for limit/cursor rules.`

- [ ] **Step 5: Xoá `ListByStatus` và `List` cũ**

Xoá hai hàm cũ khỏi `transaction_repository.go` và `settlement_repository.go`. Cập nhật các test repository còn gọi chúng:

- `TestListByStatus_ReturnsOnlyMatchingStatus`: đổi lời gọi thành `held, _, err := repo.ListByStatusPage(context.Background(), settlement.StatusHeld, 50, nil)`.
- `TestListSettlements_MostRecentFirst`: `list, _, err := repo.ListPage(context.Background(), 50, nil)`.
- `TestListSettlements_IncludesTransactionIDs`: `list, _, err := repo.ListPage(context.Background(), 50, nil)`.

Sửa comment của `transactionIDsFor` (đang nói "has one caller, a mobile app screen with no pagination yet") thành: `// One query per settlement is bounded by the page size (at most 200).`

Run: `cd services/settlement-engine && grep -rn "ListByStatus(\|\.List(ctx\|\.List(context" internal cmd`
Expected: không còn kết quả.

- [ ] **Step 6: README**

Trong `services/settlement-engine/README.md` mục `## API`, thay hai gạch đầu dòng list:

```markdown
- `GET /transactions?status=<status>[&limit=<n>][&cursor=<c>]` — `200`
  `{"items": [Transaction...], "next_cursor": "<c>" | null}`, filtered by
  status (`pending_settlement` | `held` | `settled` | `rejected`), most
  recently scored first. `400` if `status` is missing, `limit` is not an
  integer ≥ 1 (`invalid limit`), or `cursor` is not one this API issued
  (`invalid cursor`).
```

```markdown
- `GET /settlements[?limit=<n>][&cursor=<c>]` — `200`
  `{"items": [Settlement...], "next_cursor": "<c>" | null}`, most recently
  created first. Same `400`s as above for `limit` and `cursor`.
```

Và thêm ngay dưới hai dòng đó:

```markdown
Both lists are keyset-paginated: `limit` defaults to 50 and is capped at
200; pass the previous response's `next_cursor` to get the next page. A row
approved or rejected between two requests does not shift later pages.
```

- [ ] **Step 7: Chạy toàn bộ**

Run: `cd services/settlement-engine && go build ./... && go vet ./... && go test -count=1 -p 1 ./... && golangci-lint run ./...`
Expected: tất cả PASS, lint sạch.

- [ ] **Step 8: Commit**

```bash
git branch --show-current   # step/list-pagination
git add services/settlement-engine/
git commit -m "feat(settlement-engine): list endpoints page with limit and an opaque cursor"
```

---

### Task 3: OpenAPI và bên tiêu thụ trong `tests/`

**Files:**
- Modify: `docs/openapi/settlement-engine.yaml`, `tests/api/clients/settlement.py`, `tests/api/test_settlement_contract.py`, `tests/security/test_fraud_bypass.py`

**Interfaces:**
- Consumes: HTTP contract (Task 2).
- Produces: `SettlementClient.list_transactions(status, *, limit=None, cursor=None)`, `SettlementClient.list_settlements(*, limit=None, cursor=None)`.

Cần settlement-engine **đã build lại từ branch này** đang chạy ở `:8082` (cùng stack đầy đủ như `tests/README.md`). Nếu bản đang chạy là bản cũ, các test dưới đây sẽ fail vì nhận mảng.

- [ ] **Step 1: Client**

`tests/api/clients/settlement.py`, thay hai hàm:

```python
    def list_transactions(self, status: str, *, limit: int | None = None, cursor: str | None = None) -> httpx.Response:
        params = {"status": status}
        if limit is not None:
            params["limit"] = limit
        if cursor is not None:
            params["cursor"] = cursor
        return self._http.get("/transactions", params=params)
```

```python
    def list_settlements(self, *, limit: int | None = None, cursor: str | None = None) -> httpx.Response:
        params = {}
        if limit is not None:
            params["limit"] = limit
        if cursor is not None:
            params["cursor"] = cursor
        return self._http.get("/settlements", params=params)
```

- [ ] **Step 2: Contract test**

`tests/api/test_settlement_contract.py`: thay `test_list_settlements` và thêm test mới (thêm `import uuid` ở đầu file):

```python
def test_list_settlements(settlement):
    resp = settlement.list_settlements()
    assert resp.status_code == 200
    body = resp.json()
    assert isinstance(body["items"], list)
    assert body["next_cursor"] is None or isinstance(body["next_cursor"], str)


def test_list_held_transactions_pages_with_cursor(accounts, ledger, settlement):
    # Two over-threshold transactions guarantee at least two held rows.
    client_id = accounts.create_client(f"Paging {uuid.uuid4()}").json()["id"]
    debit = accounts.create_account(client_id).json()["id"]
    credit = accounts.create_account(client_id).json()["id"]
    for _ in range(2):
        ledger.post_transaction([
            {"account_id": debit, "direction": "debit", "amount": 20_000_000, "reason": "paging"},
            {"account_id": credit, "direction": "credit", "amount": 20_000_000, "reason": "paging"},
        ])

    def two_held():
        page = settlement.list_transactions("held", limit=1).json()
        return page if page["next_cursor"] else None

    first = _wait_until(two_held)
    second = settlement.list_transactions("held", limit=1, cursor=first["next_cursor"]).json()

    assert len(first["items"]) == 1 and len(second["items"]) == 1
    assert first["items"][0]["id"] != second["items"][0]["id"]
    assert first["items"][0]["scored_at"] >= second["items"][0]["scored_at"]


def test_list_endpoints_reject_bad_paging(settlement):
    for resp in (
        settlement.list_transactions("held", limit=0),
        settlement.list_settlements(limit=0),
    ):
        assert resp.status_code == 400
        assert resp.json() == {"error": "invalid limit"}
    for resp in (
        settlement.list_transactions("held", cursor="garbage!"),
        settlement.list_settlements(cursor="garbage!"),
    ):
        assert resp.status_code == 400
        assert resp.json() == {"error": "invalid cursor"}
```

Thêm helper `_wait_until` vào đầu file (cùng kiểu với `tests/security/test_fraud_bypass.py`; thêm `import time`):

```python
def _wait_until(predicate, timeout=15.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")
```

- [ ] **Step 3: `test_fraud_bypass.py`**

Dòng `for s in settlement.list_settlements().json():` đổi thành:

```python
    # The newest page covers every settlement created after this transaction
    # was held, which is the only window it could have been settled in.
    for s in settlement.list_settlements().json()["items"]:
```

- [ ] **Step 4: OpenAPI**

Trong `docs/openapi/settlement-engine.yaml`:

1. Thêm vào `components` (cùng cấp `schemas`) một khối `parameters`:

```yaml
  parameters:
    Limit:
      name: limit
      in: query
      required: false
      description: Page size. Defaults to 50; values above 200 are clamped to 200.
      schema:
        type: integer
        minimum: 1
        default: 50
    Cursor:
      name: cursor
      in: query
      required: false
      description: Opaque `next_cursor` from the previous page. Omit for the first page.
      schema:
        type: string
```

2. Thêm vào `components.schemas`:

```yaml
    TransactionPage:
      type: object
      required: [items, next_cursor]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/Transaction'
        next_cursor:
          type: ['string', 'null']
          description: Pass as `cursor` to get the next page; null on the last page.
    SettlementPage:
      type: object
      required: [items, next_cursor]
      properties:
        items:
          type: array
          items:
            $ref: '#/components/schemas/Settlement'
        next_cursor:
          type: ['string', 'null']
          description: Pass as `cursor` to get the next page; null on the last page.
```

3. `/transactions` `get`: thêm vào `parameters` hai dòng `- $ref: '#/components/parameters/Limit'` và `- $ref: '#/components/parameters/Cursor'`; schema của `'200'` thành `$ref: '#/components/schemas/TransactionPage'` với description `A page of transactions, most recently scored first`; description của `'400'` thành `Missing status, invalid limit, or invalid cursor`; description của operation thêm câu `Keyset-paginated.`

4. `/settlements` `get`: thêm `parameters:` với hai `$ref` trên; `'200'` thành `$ref: '#/components/schemas/SettlementPage'` (`A page of settlements, most recently created first`); thêm response:

```yaml
        '400':
          description: Invalid limit or invalid cursor
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
```

Kiểm tra: `python -c "import yaml; yaml.safe_load(open('docs/openapi/settlement-engine.yaml')); print('ok')"` → `ok`.

- [ ] **Step 5: Chạy**

Run (từ `tests/`, stack chạy với settlement-engine mới):

```bash
python -m pytest api -q
python -m pytest security -q -rxX
```

Expected: `api` tất cả pass; `security` đúng như trước: `76 passed, 1 deselected, 5 xfailed`.

- [ ] **Step 6: Commit**

```bash
git branch --show-current   # step/list-pagination
git add docs/openapi/settlement-engine.yaml tests/api/clients/settlement.py tests/api/test_settlement_contract.py tests/security/test_fraud_bypass.py
git commit -m "docs(openapi), tests: list endpoints return a page envelope"
```

---

### Task 4: `mobile-app` — tầng API, footer, hai màn hình

**Files:**
- Modify: `mobile-app/src/api/types.ts`, `mobile-app/src/api/settlement.ts`, `mobile-app/src/screens/HeldTransactionsScreen.tsx`, `mobile-app/src/screens/SettlementsScreen.tsx`
- Create: `mobile-app/src/components/LoadMoreFooter.tsx`, `mobile-app/src/components/LoadMoreFooter.test.tsx`
- Test: `mobile-app/src/screens/HeldTransactionsScreen.test.tsx`, `mobile-app/src/screens/SettlementsScreen.test.tsx`

**Interfaces:**
- Consumes: HTTP contract (Task 2).
- Produces: `interface Page<T> { items: T[]; next_cursor: string | null }`; `listHeldTransactions(cursor?: string): Promise<Page<Transaction>>`; `listSettlements(cursor?: string): Promise<Page<Settlement>>`; `LoadMoreFooter({ loading, failed })`.

- [ ] **Step 1: Kiểu và tầng API**

`src/api/types.ts`, thêm:

```ts
export interface Page<T> {
  items: T[];
  next_cursor: string | null;
}
```

`src/api/settlement.ts` (thay toàn bộ file):

```ts
import { env } from '../config/env';
import { fetchJson } from './http';
import type { Page, Settlement, Transaction } from './types';

function withCursor(url: string, cursor?: string): string {
  if (!cursor) return url;
  return `${url}${url.includes('?') ? '&' : '?'}cursor=${encodeURIComponent(cursor)}`;
}

export function listHeldTransactions(cursor?: string): Promise<Page<Transaction>> {
  return fetchJson<Page<Transaction>>(
    withCursor(`${env.settlementApiUrl}/transactions?status=held`, cursor),
  );
}

export function approveTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/approve`, { method: 'POST' });
}

export function rejectTransaction(id: string): Promise<void> {
  return fetchJson<void>(`${env.settlementApiUrl}/transactions/${id}/reject`, { method: 'POST' });
}

export function listSettlements(cursor?: string): Promise<Page<Settlement>> {
  return fetchJson<Page<Settlement>>(withCursor(`${env.settlementApiUrl}/settlements`, cursor));
}
```

- [ ] **Step 2: `LoadMoreFooter` (TDD)**

`src/components/LoadMoreFooter.test.tsx`:

```tsx
import { render } from '@testing-library/react-native';

import { LoadMoreFooter } from './LoadMoreFooter';

it('shows a loading line while the next page loads', async () => {
  const { getByText } = await render(<LoadMoreFooter loading failed={false} />);
  expect(getByText('Loading more…')).toBeTruthy();
});

it('shows a failure line when the next page failed', async () => {
  const { getByText } = await render(<LoadMoreFooter loading={false} failed />);
  expect(getByText('Failed to load more.')).toBeTruthy();
});

it('renders nothing otherwise', async () => {
  const { toJSON } = await render(<LoadMoreFooter loading={false} failed={false} />);
  expect(toJSON()).toBeNull();
});
```

Run: `cd mobile-app && npx jest src/components/LoadMoreFooter.test.tsx` → FAIL (`Cannot find module './LoadMoreFooter'`).

`src/components/LoadMoreFooter.tsx`:

```tsx
import { StyleSheet, Text } from 'react-native';

import { colors, spacing, typography } from '../theme';

interface Props {
  loading: boolean;
  failed: boolean;
}

export function LoadMoreFooter({ loading, failed }: Props) {
  if (loading) {
    return <Text style={[typography.caption, styles.text]}>Loading more…</Text>;
  }
  if (failed) {
    return <Text style={[typography.caption, styles.text, styles.error]}>Failed to load more.</Text>;
  }
  return null;
}

const styles = StyleSheet.create({
  text: { padding: spacing.md, textAlign: 'center', color: colors.textSecondary },
  error: { color: colors.danger },
});
```

Run lại → 3 PASS.

- [ ] **Step 3: Cập nhật test của hai màn hình (fail trước)**

`src/screens/HeldTransactionsScreen.test.tsx`:
- Thêm helper ở đầu file (dưới `renderWithQuery`):

```tsx
const page = <T,>(items: T[], next_cursor: string | null = null) => ({ items, next_cursor });

const heldTx = (id: string, amount: number) => ({
  id,
  amount,
  score: 0.9,
  decision: 'hold',
  status: 'held',
  triggered_rules: [],
  scored_at: '',
});
```

- Mọi chỗ `listHeldTransactions` được mock bằng một mảng (`mockResolvedValue([...])`, `mockResolvedValueOnce([...])`) đổi thành mảng đó bọc trong `page(...)`, ví dụ `.mockResolvedValueOnce(page([ ... ]))` và `.mockResolvedValueOnce(page([]))`. Không đổi gì khác trong các test cũ.
- Thêm ba test:

```tsx
it('loads the next page with its cursor when the list reaches the end', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(page([heldTx('txn-1', 500)], 'cursor-2'))
    .mockResolvedValueOnce(page([heldTx('txn-2', 200)]));

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(await findByTestId('held-tx-txn-2')).toBeTruthy();
  expect(getByTestId('held-tx-txn-1')).toBeTruthy();
  expect((settlementApi.listHeldTransactions as jest.Mock).mock.calls[1][0]).toBe('cursor-2');
});

it('does not request another page when there is no next cursor', async () => {
  (settlementApi.listHeldTransactions as jest.Mock).mockResolvedValue(page([heldTx('txn-1', 500)]));

  const { findByTestId, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(settlementApi.listHeldTransactions).toHaveBeenCalledTimes(1);
});

it('keeps loaded rows and shows a footer when the next page fails', async () => {
  (settlementApi.listHeldTransactions as jest.Mock)
    .mockResolvedValueOnce(page([heldTx('txn-1', 500)], 'cursor-2'))
    .mockRejectedValueOnce(new Error('network error'));

  const { findByTestId, findByText, getByTestId } = await renderWithQuery(<HeldTransactionsScreen />);
  await findByTestId('held-tx-txn-1');

  await fireEvent(getByTestId('held-list'), 'onEndReached');

  expect(await findByText('Failed to load more.')).toBeTruthy();
  expect(getByTestId('held-tx-txn-1')).toBeTruthy();
});
```

`src/screens/SettlementsScreen.test.tsx`:
- Thêm `fireEvent` vào import từ `@testing-library/react-native`, và helper `page` y như trên.
- Mọi mock `listSettlements` bằng mảng đổi thành `page([...])`.
- Thêm test:

```tsx
it('loads the next page of settlements with its cursor', async () => {
  const settlementRow = (id: string) => ({
    id,
    transaction_ids: ['txn-1'],
    transaction_count: 1,
    total_amount: 100,
    created_at: '2026-08-20T00:00:00Z',
  });
  (settlementApi.listSettlements as jest.Mock)
    .mockResolvedValueOnce(page([settlementRow('settle-1')], 'cursor-2'))
    .mockResolvedValueOnce(page([settlementRow('settle-2')]));

  const { findByTestId, getByTestId } = await renderWithQuery(<SettlementsScreen />);
  await findByTestId('settlement-card-settle-1');

  await fireEvent(getByTestId('settlements-list'), 'onEndReached');

  expect(await findByTestId('settlement-card-settle-2')).toBeTruthy();
  expect((settlementApi.listSettlements as jest.Mock).mock.calls[1][0]).toBe('cursor-2');
});
```

Run: `cd mobile-app && npx jest src/screens/HeldTransactionsScreen.test.tsx src/screens/SettlementsScreen.test.tsx`
Expected: FAIL — các màn hình vẫn đọc mảng (`data.map is not a function` hoặc không tìm thấy testID `held-list`).

- [ ] **Step 4: `HeldTransactionsScreen.tsx`**

Thay import `useQuery` bằng `useInfiniteQuery` (giữ `useMutation`, `useQueryClient`), thêm `import { LoadMoreFooter } from '../components/LoadMoreFooter';`. Thay khối `useQuery` bằng:

```tsx
  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isFetchNextPageError,
  } = useInfiniteQuery({
    queryKey: ['held-transactions'],
    queryFn: ({ pageParam }) => listHeldTransactions(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const items = data?.pages.flatMap((p) => p.items) ?? [];
```

Điều kiện lỗi toàn màn hình đổi từ `if (error)` thành `if (error && !data)` (lỗi của trang sau không được thay cả màn hình). Trên `FlatList`:

```tsx
      <FlatList
        testID="held-list"
        data={items}
        keyExtractor={(t) => t.id}
        onEndReached={() => {
          if (hasNextPage && !isFetchingNextPage) {
            void fetchNextPage();
          }
        }}
        onEndReachedThreshold={0.5}
        ListFooterComponent={
          <LoadMoreFooter loading={isFetchingNextPage} failed={isFetchNextPageError} />
        }
```

Các prop còn lại (`ListHeaderComponent`, `renderItem`, `ListEmptyComponent`) giữ nguyên. `invalidate` vẫn là `queryClient.invalidateQueries({ queryKey: ['held-transactions'] })`.

- [ ] **Step 5: `SettlementsScreen.tsx`**

Tương tự: `useInfiniteQuery` với `queryKey: ['settlements']`, `queryFn: ({ pageParam }) => listSettlements(pageParam)`, cùng `initialPageParam`/`getNextPageParam`; `items = data?.pages.flatMap((p) => p.items) ?? []`; `if (error && !data)`; `FlatList` có `testID="settlements-list"`, `data={items}`, cùng `onEndReached`, `onEndReachedThreshold={0.5}` và `ListFooterComponent` như Step 4. `renderItem` và `ListEmptyComponent` giữ nguyên. Thêm import `LoadMoreFooter`.

- [ ] **Step 6: Chạy toàn bộ**

Run (từ `mobile-app/`): `npx jest src/screens src/components && npm test -- --ci && npm run lint && npm run format:check`
Expected: tất cả pass, lint/format sạch (nếu Prettier báo, `npx prettier --write` đúng các file đã sửa).

- [ ] **Step 7: Commit**

```bash
git branch --show-current   # step/list-pagination
git add mobile-app/src/api/types.ts mobile-app/src/api/settlement.ts mobile-app/src/components/LoadMoreFooter.tsx mobile-app/src/components/LoadMoreFooter.test.tsx mobile-app/src/screens/HeldTransactionsScreen.tsx mobile-app/src/screens/SettlementsScreen.tsx mobile-app/src/screens/HeldTransactionsScreen.test.tsx mobile-app/src/screens/SettlementsScreen.test.tsx
git commit -m "feat(mobile-app): Held and Settlements tabs load page by page"
```

---

### Task 5: Đo lại trên dữ liệu local

**Files:** không có (chỉ kiểm chứng; kết quả ghi vào báo cáo task và mô tả PR).

- [ ] **Step 1: Build và chạy lại settlement-engine từ branch**

```bash
cd services/settlement-engine && go build -o <bin>/settlement-engine.exe ./cmd/server
```

Dừng tiến trình đang nghe `:8082`, chạy bản mới với cùng env (`DATABASE_URL=postgres://settlement:settlement@localhost:5435/settlement?sslmode=disable`, `NATS_URL=nats://localhost:4222`, `SETTLEMENT_BATCH_INTERVAL_SECONDS=5`). Migration 000010 chạy lúc khởi động.

- [ ] **Step 2: Đo**

```bash
docker exec settleguard-settlement-dev psql -U settlement -tAc "select count(*) from transactions where status='held'"
curl -s -o /dev/null -w "held page 1: %{time_total}s %{size_download}B\n" "localhost:8082/transactions?status=held"
c=$(curl -s "localhost:8082/transactions?status=held" | python -c 'import json,sys; print(json.load(sys.stdin)["next_cursor"])')
curl -s -o /dev/null -w "held page 2: %{time_total}s\n" "localhost:8082/transactions?status=held&cursor=$c"
curl -s -o /dev/null -w "settlements page 1: %{time_total}s\n" "localhost:8082/settlements"
docker exec settleguard-settlement-dev psql -U settlement -c "EXPLAIN SELECT id FROM transactions WHERE status='held' ORDER BY scored_at DESC, id DESC LIMIT 51"
```

Expected: trang 1 và trang 2 của held dưới 1 s (trước: 22,9 s, 12,2 MB); `EXPLAIN` dùng `idx_transactions_status_scored` (Index Scan), không có `Sort` trên toàn bảng.

- [ ] **Step 3: Tab Held trên bản web**

Build lại bản web trỏ gateway (lệnh trong `tests/e2e/README.md` trên branch `step/playwright-e2e`, hoặc: `EXPO_PUBLIC_*_API_URL=http://localhost:8000/<prefix> npx expo export --platform web --output-dir ../.e2e-web` từ `mobile-app/`), serve trên `:8090`, mở trong trình duyệt hoặc chạy một script Playwright ngắn: tab Held phải hiện dòng `held-tx-*` đầu tiên trong vài giây. Ghi thời gian vào báo cáo.

---

## Self-Review

**Spec coverage:**

| Spec | Task |
|---|---|
| §2 contract, thứ tự, `limit`, `cursor`, `limit+1`, keyset | 1 (query), 2 (HTTP) |
| §2 index | 1 |
| §2 vì sao envelope | (lý do nằm trong spec; code comment ở `cursorWire`) |
| §3 app: `Page<T>`, API cursor, `useInfiniteQuery`, footer, testID mới, approve/reject | 4 |
| §4 một branch `step/list-pagination`, thứ tự commit | Global Constraints, 1→4 |
| §5 bên tiêu thụ: app, `tests/api`, fraud bypass, OpenAPI, README | 2, 3, 4 |
| §5 plan Playwright Task 4 | để khi tiếp tục dự án con 3 (ngoài plan này) |
| §6 test Go (ties, walk, single page, 400, default/max) | 1, 2 |
| §6 contract test, security không đổi, Jest | 3, 4 |
| §6 đo trên dữ liệu local, tab Held trên web | 5 |
