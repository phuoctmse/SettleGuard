# accounts-service MVP — Thiết kế

Ngày: 2026-08-08
Trạng thái: Đã duyệt (đang chờ review spec cuối cùng)

## 1. Bối cảnh & Phạm vi

`ledger-service` (đã merge vào `main`) hiện ghi các bút toán double-entry
tham chiếu tới `account_id` dạng UUID trần — không có gì trong hệ thống
định nghĩa account đó thuộc về ai. `accounts-service` lấp khoảng trống này:
sở hữu định danh party/account theo đúng vai trò được charter mô tả
(`docs/superpowers/specs/2026-07-29-project-charter-design.md`).

MVP này **chỉ** làm identity + status. Balance-of-obligation (số dư nghĩa
vụ) mà charter gán cho accounts-service bị hoãn lại — nó phụ thuộc vào
event broker (`account.updated`, `ledger.entry-recorded`), thứ chưa được
chọn. Field này sẽ được thêm khi event broker được wire vào (bước kế tiếp
trong roadmap, xem "Để lại cho việc khác" bên dưới).

Coding mode: subagent-driven (không phải mentor mode) — service này lặp
pattern CRUD đơn giản, không có logic nghiệp vụ phức tạp cần dạy.

Ngoại lệ: business rule liên quan tới `status` (chặn tạo `Account` khi
`ClientBusiness` cha đang `suspended`, và mọi state transition qua
`PATCH .../status`) phải được tự đọc kỹ diff và xác nhận trước khi merge
— đây là phần duy nhất trong MVP này ảnh hưởng tới balance-of-obligation
sau này, nên không chỉ dựa vào subagent tự báo cáo test pass.

## 2. Kiến trúc & Tech Stack

Dùng lại nguyên pattern của `ledger-service` để nhất quán toàn hệ thống:

- Layout chuẩn: `cmd/server/main.go` + `internal/{api,db,account,testutil}/`
- `net/http` + chi v5 router
- pgx v5 (Postgres driver qua `database/sql`)
- golang-migrate v4 — file `.sql` thuần, không code-gen
- testify cho assertion, testcontainers-go cho test chạm Postgres thật
- Module path: `github.com/phuoctmse/settleguard/accounts-service`
- Postgres schema riêng (`accounts`), không chia sẻ với `ledger-service`

## 3. Domain Model

### ClientBusiness (tenant)

Tổ chức tích hợp qua API (vd một sàn thương mại điện tử). Sở hữu các
`Account` bên dưới nó.

| Field      | Type      | Ghi chú                                |
|------------|-----------|-----------------------------------------|
| id         | UUID      | PK                                      |
| name       | text      | bắt buộc, không rỗng                    |
| status     | text      | `active` \| `suspended`                 |
| created_at | timestamptz | default now()                         |

### Account

Đại diện cho end-user của một `ClientBusiness` — đơn vị nắm giữ obligation
mà `ledger-service` ghi bút toán vào.

| Field        | Type      | Ghi chú                                          |
|--------------|-----------|---------------------------------------------------|
| id           | UUID      | PK — chính là `account_id` mà ledger-service dùng |
| client_id    | UUID      | FK → ClientBusiness.id, bắt buộc                  |
| external_ref | text      | optional — id nội bộ mà client tự quản lý cho end-user của họ |
| status       | text      | `active` \| `suspended` \| `closed`               |
| created_at   | timestamptz | default now()                                   |
| updated_at   | timestamptz | default now(), cập nhật khi status đổi          |

### Business rule

Tạo `Account` mới bị chặn (`422`) nếu `ClientBusiness` cha đang
`suspended`. Tạo `Account` dưới `client_id` không tồn tại trả `404`.

Chuyển trạng thái (`PATCH .../status`) không bị giới hạn theo một state
machine trong MVP này — bất kỳ giá trị status hợp lệ nào cũng được chấp
nhận (kể cả `closed` → `active`). Việc ràng buộc thứ tự chuyển trạng thái
hợp lý (nếu cần) để lại cho sau, khi có use case cụ thể đòi hỏi.

## 4. HTTP API

- `POST /clients` — body `{"name": "<string>"}` → `201` ClientBusiness.
  `400` nếu `name` rỗng.
- `GET /clients/{id}` — `200` ClientBusiness hoặc `404`.
- `PATCH /clients/{id}/status` — body `{"status": "active"|"suspended"}` →
  `200` ClientBusiness cập nhật, `400` nếu status không hợp lệ, `404` nếu
  không tồn tại.
- `POST /accounts` — body `{"client_id": "<uuid>", "external_ref": "<string, optional>"}`
  → `201` Account. `400` nếu `client_id` không phải UUID hợp lệ, `404` nếu
  client không tồn tại, `422` nếu client đang `suspended`.
- `GET /accounts/{id}` — `200` Account hoặc `404`.
- `GET /accounts?client_id=<uuid>` — `200` danh sách Account thuộc client
  đó (rỗng nếu không có, không phải lỗi). `400` nếu thiếu `client_id` hoặc
  không phải UUID hợp lệ.
- `PATCH /accounts/{id}/status` — body
  `{"status": "active"|"suspended"|"closed"}` → `200` Account cập nhật,
  `400` nếu status không hợp lệ, `404` nếu không tồn tại.
- `GET /health` — health check, giống ledger-service.

## 5. Testing

Giống hệt cấu trúc test của `ledger-service`:

- `internal/account` — test thuần Go, không cần DB: validate status hợp
  lệ, business rule "không tạo account dưới client suspended" (test ở tầng
  domain/repository logic, không phụ thuộc HTTP).
- `internal/db` — test migration chạy đúng, dùng testcontainers-go.
- `internal/api` — test handler end-to-end qua `httptest.NewServer`, dùng
  testcontainers-go cho Postgres thật.
- TDD theo từng task: viết test fail trước, implement, xác nhận pass.

## 6. Tiêu chí thành công cho MVP này

- `POST /clients` rồi `POST /accounts` với `client_id` đó tạo được account
  thành công.
- `POST /accounts` dưới client `suspended` trả `422`; dưới client không
  tồn tại trả `404`.
- `GET /accounts?client_id=...` trả đúng danh sách account của client đó.
- Toàn bộ test pass với `go test ./...` (yêu cầu Docker); `go vet ./...`
  sạch; `go build ./...` thành công.

## 7. Để lại cho việc khác (không nằm trong spec này)

- Balance-of-obligation trên Account — cần event broker
  (`ledger.entry-recorded` → cập nhật balance) — quyết định ở bước "Event
  broker decision" kế tiếp trong roadmap.
- Publish sự kiện `account.updated` — cùng phụ thuộc event broker.
- Authentication/authorization cho HTTP API (client business tự xác thực
  qua API key) — giống ledger-service, hoãn tới khi có quyết định chung về
  auth.
- Dockerfile / k8s manifest cho accounts-service.
- Union "party type" tổng quát (end-user vs business như một entity thống
  nhất) — MVP này dùng model cụ thể ClientBusiness + Account thay vì một
  Party trừu tượng, vì đơn giản hơn và đủ cho nhu cầu hiện tại.
