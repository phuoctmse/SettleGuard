# Phân trang cho danh sách held transactions và settlements — Thiết kế

Ngày: 2026-10-18
Trạng thái: Nháp — cần review trước khi chuyển thành implementation plan

## 1. Bối cảnh & Phạm vi

`GET /transactions?status=held` và `GET /settlements` của settlement-engine trả
**toàn bộ** bản ghi, không giới hạn. Tab Held và tab Settlements của
`mobile-app` tải nguyên danh sách đó trong một request.

Đo trên stack local ngày 2026-10-18 (DB settlement có 43.748 giao dịch `held`
— dữ liệu tích luỹ từ lần đo Locust (#29) và các lần chạy pentest — 3.048
`settled`, 105 settlement):

| Endpoint | Thời gian | Kích thước |
|---|---|---|
| `GET /transactions?status=held` | 22,9 s | 12,2 MB |
| `GET /settlements` | 0,08 s | 135 KB |

Tab Held kẹt ở "Loading" và chặn Task 3 của dự án con 3 (Playwright E2E,
`docs/superpowers/plans/2026-10-18-playwright-e2e.md`). CI không gặp vì DB luôn
mới, nhưng bất kỳ môi trường sống lâu nào cũng sẽ gặp. Settlements nhỏ hôm nay
nhưng tăng mỗi chu kỳ batch, cùng một khuyết tật.

**Trong phạm vi:** phân trang keyset cho hai endpoint trên; tab Held và tab
Settlements cuộn tới đâu tải tới đó; cập nhật mọi bên tiêu thụ trong repo.

**Ngoài phạm vi:**
- Bộ lọc, tìm kiếm.
- notification-service — đã có `limit` (mặc định 50, tối đa 200).
- Dọn 43.748 dòng `held` ở local: giữ lại làm dữ liệu đo.
- Gateway — đường dẫn không đổi.

## 2. Contract API (settlement-engine)

```
GET /transactions?status=held&limit=50&cursor=<opaque>
GET /settlements?limit=50&cursor=<opaque>

200 {"items": [ ...phần tử như hiện nay... ], "next_cursor": "<opaque>" | null}
```

- **Thứ tự:** `/transactions` theo `scored_at DESC, id DESC`; `/settlements`
  theo `created_at DESC, id DESC`. `id` làm thứ tự xác định khi trùng thời điểm.
- **`limit`:** mặc định 50. Lớn hơn 200 → hạ về 200 (giống notification-service).
  Nhỏ hơn 1 hoặc không phải số nguyên → `400 {"error":"invalid limit"}`. Kiểm
  tra cả cận dưới — bài học SG-PT-07 (`docs/security/PENTEST_REPORT.md`).
- **`cursor`:** base64url (không padding) của JSON
  `{"t":"<RFC3339Nano>","id":"<uuid>"}` — timestamp đủ micro giây và id của
  phần tử cuối trang trước. Với client là chuỗi opaque. Không giải mã được, JSON
  sai, hoặc `t`/`id` không hợp lệ → `400 {"error":"invalid cursor"}`.
- **Có trang sau không:** lấy `limit + 1` dòng; có dòng dư thì trả `next_cursor`
  (mã hoá phần tử thứ `limit`), không thì `null`. Không `COUNT(*)`.
- **Keyset:** `WHERE (scored_at, id) < ($t, $id)` (tương tự với `created_at`).
  Approve/reject giữa hai lần cuộn không làm dịch các trang sau — đây là lý do
  không dùng offset.
- **Index (migration mới):**
  `CREATE INDEX idx_transactions_status_scored ON transactions (status, scored_at DESC, id DESC);`
  `CREATE INDEX idx_settlements_created ON settlements (created_at DESC, id DESC);`
- `transaction_ids` của mỗi settlement vẫn lấy bằng một truy vấn mỗi settlement
  (#34); giờ tối đa 200 truy vấn mỗi trang.
- `status` vẫn bắt buộc trên `/transactions` (400 khi thiếu, như hiện nay).

### Vì sao envelope, không phải các phương án khác

- **Giữ mảng, client tự tạo cursor từ phần tử cuối:** API trả `scored_at` ở độ
  chính xác mili giây (`timeFormat`), Postgres lưu micro giây — hai giao dịch
  trong cùng một mili giây bị bỏ sót hoặc lặp khi cuộn. Sửa định dạng thời gian
  cũng là đổi contract.
- **Cursor trong header `X-Next-Cursor`:** gateway chỉ expose `X-Request-Id`
  qua CORS (`services/gateway/internal/api/router.go`), trình duyệt không đọc
  được header khác.
- **Offset (`?page=`):** danh sách Held thay đổi liên tục khi người vận hành
  duyệt — các dòng dịch chỗ, cuộn sẽ bỏ sót hoặc lặp.

Envelope làm vỡ hình dạng response; mọi bên tiêu thụ nằm trong repo và được sửa
trong cùng thay đổi (§5). Chưa có client bên ngoài.

## 3. App (`mobile-app`)

- `src/api/types.ts`: `export interface Page<T> { items: T[]; next_cursor: string | null }`.
- `src/api/settlement.ts`: `listHeldTransactions(cursor?: string): Promise<Page<Transaction>>`
  và `listSettlements(cursor?: string): Promise<Page<Settlement>>`; có cursor
  thì nối `cursor=<encodeURIComponent(cursor)>` vào query. App không gửi
  `limit` (dùng mặc định 50).
- `HeldTransactionsScreen`, `SettlementsScreen`: `useQuery` → `useInfiniteQuery`:

  ```ts
  useInfiniteQuery({
    queryKey: ['held-transactions'],
    queryFn: ({ pageParam }) => listHeldTransactions(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  })
  ```

  `FlatList` nhận `data.pages.flatMap((p) => p.items)`; `onEndReached` gọi
  `fetchNextPage()` chỉ khi `hasNextPage && !isFetching` — `fetchNextPage` mặc định
  huỷ refetch đang chạy (react-query v5), nên không được gọi trong lúc refetch sau approve/reject.
- Footer danh sách: `"Loading more…"` khi đang tải trang tiếp; `"Failed to load
  more."` khi trang tiếp lỗi — giữ nguyên các dòng đã có, lần `onEndReached`
  sau thử lại.
- Approve/reject giữ `invalidateQueries(['held-transactions'])`; react-query v5
  refetch tuần tự các trang đã tải, mỗi trang lấy cursor từ trang trước.
- testID giữ nguyên toàn bộ; thêm `held-list`, `settlements-list` trên `FlatList`.
- Không đổi text hiển thị, style, điều hướng; tab Alerts và Accounts không đổi.

## 4. Branch và thứ tự

Một branch repo-level **`step/list-pagination`** chứa cả settlement-engine,
`mobile-app`, `tests/` và OpenAPI. Đây là ngoại lệ có lý do với quy ước "một
branch mỗi service": contract đổi hình dạng, nếu backend merge trước app thì
`main` chứa một app vỡ — trái với "`main` luôn chạy được" (`CLAUDE.md` § Git
Workflow). Gộp nhà cung cấp và mọi bên tiêu thụ thì CI và `main` xanh ở mọi thời
điểm. Tên `step/<mô tả>` là quy ước sẵn có cho thay đổi xuyên repo.

Thứ tự commit:
1. Migration index + repository keyset (testcontainers).
2. Handler: `limit`, `cursor`, envelope, 400.
3. `docs/openapi/settlement-engine.yaml` + bên tiêu thụ trong `tests/`.
4. `mobile-app`: tầng API + hai màn hình.

## 5. Bên tiêu thụ phải sửa

| Nơi | Thay đổi |
|---|---|
| `mobile-app/src/api/settlement.ts`, `types.ts`, hai screen + test | §3 |
| `tests/api/clients/settlement.py` | `list_transactions(status, limit=None, cursor=None)`, `list_settlements(limit=None, cursor=None)` |
| `tests/api/test_settlement_contract.py` | khẳng định envelope; đi qua 2 trang với `limit=1` |
| `tests/security/test_fraud_bypass.py` | vòng lặp settlement đọc `["items"]` |
| `docs/openapi/settlement-engine.yaml` | tham số `limit`/`cursor`, schema envelope, response 400 |
| `docs/superpowers/plans/2026-10-18-playwright-e2e.md` Task 4 (khi tiếp tục) | `find_settlement` đọc `list_settlements().json()["items"]` |

`services/settlement-engine/README.md` (dòng ~109–121 mô tả hai endpoint là
"list"): cập nhật theo §2 — `limit`, `cursor`, envelope, 400.

## 6. Kiểm thử & tiêu chí xong

- **Go, repository** (testcontainers, Postgres thật; repository nhận `limit`
  và cursor đã giải mã, không tự áp mặc định): trả đúng `limit` dòng và báo
  còn trang sau; thứ tự `DESC`; đi hết các trang
  bằng cursor thấy đủ mọi dòng, không trùng, không sót; **hai dòng cùng
  `scored_at` (và hai settlement cùng `created_at`) được tách đúng bằng `id`**;
  chỉ trả đúng `status` được hỏi.
- **Go, handler:** không có `limit` → 50; `limit=500` → 200; envelope đúng
  hình dạng; `next_cursor` `null` ở trang cuối;
  `limit=0`, `limit=-1`, `limit=abc`, cursor hỏng → 400 với đúng thông điệp.
- **Contract (`tests/api`):** envelope; 2 trang `limit=1` khác nhau và đúng thứ tự.
- **`tests/security`** vẫn cho đúng kết quả cũ (76 passed, 5 xfailed).
- **Jest:** mock trả envelope; `onEndReached` → gọi API với `next_cursor` và
  hiện dòng trang 2; `next_cursor` `null` → không gọi thêm; trang 2 lỗi → giữ
  dòng trang 1, hiện `"Failed to load more."`. `npm test`, lint, format pass.
- **Đo lại trên dữ liệu local** (43.748 dòng `held`): `GET /transactions?status=held`
  trang đầu dưới 1 s (trước: 22,9 s); tab Held trên bản web hiện dòng đầu.
- **CI:** mọi job xanh trên PR.
- Sau merge: tiếp tục dự án con 3 từ Task 3 (commit WIP `afaf568` trên
  `step/playwright-e2e`).
