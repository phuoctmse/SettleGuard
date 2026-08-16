# notification-service MVP — Thiết kế

Ngày: 2026-08-16
Trạng thái: Đã duyệt (đang chờ review spec cuối cùng)

## 1. Bối cảnh & Phạm vi

`settlement-engine` (đã merge vào `service/ledger-service`) publish
`transaction.risk-scored` (mọi giao dịch, cả pass lẫn hold) và
`settlement.finalized` (mỗi lần chạy batch) lên stream `SETTLEMENT_EVENTS`.
Chưa có gì trong hệ thống tiêu thụ hai event này để cảnh báo con người —
đây là khoảng trống `notification-service` lấp vào, đúng vai trò charter mô
tả: "subscribes to risk-hold and settlement-finalized events; sends
push/email alerts. Never called synchronously by other services."

MVP này **chỉ** làm phần consume + lưu trữ (log/DB). Delivery channel thật
(push, email) bị hoãn lại có chủ đích:

- **Push** cần `mobile-app` (chưa tồn tại — chưa scaffold) làm đích nhận.
- **Email** cần chọn provider (SMTP/SendGrid/SES...) — chưa quyết định,
  chưa có credentials.
- Quan trọng hơn: hiện **không có contact info nào** trong hệ thống.
  `client_businesses` và `accounts` (accounts-service) không có cột email;
  payload của `transaction.risk-scored`/`settlement.finalized` cũng chỉ
  mang UUID (`account_ids`, `transaction_ids`), không mang danh tính liên
  hệ. Cố gắng gửi email/push thật ở bước này sẽ phải tự chế một trường dữ
  liệu chưa ai yêu cầu.

Vì vậy v1 ghi mỗi alert thành một row trong bảng `notifications` (nội dung
đầy đủ ở `payload` JSONB) — đóng vai trò audit trail kiêm điểm cắm sẵn cho
delivery channel thật khi mobile-app/email provider được chọn.

Coding mode: Claude code trực tiếp (theo working-style hiện tại của dự án,
không phải mentor mode).

## 2. Kiến trúc & Tech Stack

Service Python đầu tiên có code trong repo — không có tiền lệ Python nội
bộ để theo, nên các lựa chọn dưới đây ưu tiên giữ tinh thần "đơn giản,
không framework/ORM nặng" mà 3 service Go đã theo, dịch sang hệ sinh thái
Python:

- Layout: `main.py` (entrypoint) + `internal/{api,broker,db,consumer,notifications}/`
  (mirror `cmd/server/main.go` + `internal/{api,broker,db,...}/` bên Go)
- `psycopg` v3 — driver Postgres thuần, không ORM (không SQLAlchemy)
- `migrate` CLI (golang-migrate) — file `.sql` thuần, không Alembic; chạy
  qua subprocess ở startup để mirror hành vi tự-migrate-khi-boot của các
  service Go (vốn dùng golang-migrate như thư viện Go, không cần binary
  CLI ngoài) — Python không có thư viện tương đương nên gọi CLI thay
- `nats-py` (`nats.js`) — client NATS JetStream chính thức, tương đương
  `nats.go` bên Go
- `http.server` (stdlib) — chỉ cần một endpoint `/health`, không cần
  FastAPI/Flask vì service này không bị service khác gọi synchronous
- `pytest` + `testcontainers-python` — Postgres thật (module chính thức
  `testcontainers.postgres`) và NATS thật (generic `DockerContainer` chạy
  `nats:2.10-alpine -js`, khớp version dùng trong `docker-compose.yml`) —
  không mock DB/broker, giữ nguyên convention "no mocks" của repo
- Postgres schema riêng (`notification`), không chia sẻ với service khác

## 3. Domain Model

### Notification

Một bản ghi cảnh báo đã được tạo ra từ một event risk-hold hoặc
settlement-finalized.

| Field      | Type        | Ghi chú                                          |
|------------|-------------|---------------------------------------------------|
| id         | UUID        | PK                                                 |
| type       | text        | `risk_hold` \| `settlement_finalized`              |
| subject_id | UUID        | `transaction_id` (risk_hold) hoặc `settlement_id` (settlement_finalized) |
| payload    | JSONB       | toàn bộ body của event gốc — đủ dữ liệu để render alert thật sau này |
| created_at | timestamptz | default now()                                      |

Ràng buộc: `UNIQUE (type, subject_id)`.

Bảng này tự nó vừa là bản ghi nghiệp vụ vừa là cơ chế idempotency — không
cần bảng dedup riêng như `processed_ledger_transactions` bên
accounts-service, vì `(type, subject_id)` vốn đã là khoá tự nhiên duy nhất
theo nghiệp vụ (một transaction chỉ có tối đa một risk-hold notification,
một settlement chỉ finalize một lần).

## 4. Xử lý event

Hai durable JetStream consumer trên stream `SETTLEMENT_EVENTS`, mỗi
consumer filter một subject riêng:
`notification-service-risk-hold` (filter `transaction.risk-scored`) và
`notification-service-settlement-finalized` (filter
`settlement.finalized`). Ban đầu định làm một consumer duy nhất filter cả
2 subject, nhưng `nats-py` (client legacy `nats.js`, xem `ConsumerConfig`)
chỉ hỗ trợ `filter_subject` số ít, không có `filter_subjects` số nhiều
như NATS server 2.10+ hỗ trợ — tách 2 consumer vừa tránh phụ thuộc vào
tính năng không chắc client hỗ trợ, vừa khớp đúng pattern
một-consumer-một-subject mà 2 consumer Go hiện có trong hệ thống
(`ledger-service`, `accounts-service`, `settlement-engine`) đều đang theo.
Cả hai đều dùng `DeliverAllPolicy` (nhất quán với các consumer Go) — muốn
có đủ lịch sử hold/settlement kể cả khi service này khởi động sau các
event đã được publish.

Logic `handle_message`:

1. Parse payload theo subject của message.
2. Nếu subject là `transaction.risk-scored` và `decision != "hold"` →
   Ack, không ghi gì (không tạo notification cho giao dịch pass — đúng
   phạm vi charter "risk **holds**", không phải mọi giao dịch đã chấm
   điểm).
3. Ngược lại → `INSERT INTO notifications (...) ON CONFLICT (type,
   subject_id) DO NOTHING`, rồi Ack.

Đây là invariant nghiệp vụ mới, sẽ thêm **NOTIFICATION-01** (chỉ tạo
notification khi risk-scored có `decision=hold`, bỏ qua `pass`) vào
`docs/BUSINESS_RULES.md` khi implement.

Không có logic time-window nào trong service này, nên rủi ro kiểu
`SETTLEMENT-02b` (event-time vs processing-time trên `DeliverAllPolicy`
replay) không áp dụng ở đây — đã cân nhắc và loại trừ tường minh.

Không có outbox pattern (CROSS-02) trong service này: notification-service
không publish event nào cho consumer khác — nó là điểm cuối (terminal
consumer) của luồng sự kiện trong v1.

## 5. Testing

- `internal/notifications` — test repository chạm Postgres thật
  (`testcontainers.postgres`): insert thành công, `ON CONFLICT` không tạo
  row trùng khi cùng `(type, subject_id)`.
- `internal/consumer` — test `handle_message` với NATS thật
  (generic `DockerContainer` chạy `nats:2.10-alpine -js`): publish message
  giả lập `transaction.risk-scored` (case `hold` → có row, case `pass` →
  không có row) và `settlement.finalized` (→ có row); publish trùng message
  → vẫn chỉ một row.
- TDD theo từng task: viết test fail trước, implement, xác nhận pass.

## 6. Tiêu chí thành công cho MVP này

- Publish một `transaction.risk-scored` với `decision="hold"` lên
  `SETTLEMENT_EVENTS` → xuất hiện đúng một row `type=risk_hold` trong
  `notifications` sau khi service tiêu thụ.
- Publish `transaction.risk-scored` với `decision="pass"` → không tạo row
  nào.
- Publish `settlement.finalized` → xuất hiện đúng một row
  `type=settlement_finalized`.
- Publish lại (redeliver) cùng một event → không tạo row trùng.
- Toàn bộ test pass với `pytest` (yêu cầu Docker); service khởi động thành
  công qua `docker-compose` (thêm `notification-postgres`, port `5436`,
  theo đúng pattern 3 service Postgres hiện có trong
  `infra/docker/docker-compose.yml`).

## 7. Để lại cho việc khác (không nằm trong spec này)

- Delivery channel thật (email/push) — cần chọn provider + có địa chỉ
  liên hệ thật trong hệ thống (accounts-service/client_businesses hiện
  chưa có cột email/push token nào).
- Endpoint HTTP để ops team query lịch sử notification (vd
  `GET /notifications?client_id=...`) — chưa có use case cụ thể yêu cầu
  ở v1.
- Authentication/authorization — giống các service khác, hoãn tới khi có
  quyết định chung về auth.
- Dockerfile / k8s manifest cho notification-service.
- Retry/backoff riêng cho gửi thật khi có delivery channel — hiện tại chỉ
  cần Ack sau khi ghi DB thành công, NATS JetStream tự redeliver nếu
  Nak/timeout.
