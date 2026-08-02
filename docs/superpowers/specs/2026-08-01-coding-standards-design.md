# Thiết kế Coding Standards

**Ngày:** 2026-08-01
**Trạng thái:** Đã duyệt để triển khai

## Mục đích

SettleGuard hiện chưa có tài liệu coding standards chung cho toàn repo.
`CLAUDE.md` đã bao quát *stack nào* mỗi service dùng (mục Stack), *test
được tổ chức ra sao* (Testing Layout), và *branch/commit/merge hoạt động
thế nào* (Git Workflow) — nhưng chưa có gì về quy ước đặt tên, định dạng
code, hay cách xử lý lỗi trong chính code. Khi công việc triển khai thực
tế bắt đầu (ledger-service là service đầu tiên được xây dựng), các quyết
định này cần được nêu rõ ràng thay vì tùy tiện quyết theo từng file.

Spec này định nghĩa một file mới `docs/CODING_STANDARDS.md`, tự thân đầy
đủ (không chỉ trỏ tới style guide bên ngoài), bao gồm hai mảng: đặt tên &
định dạng, và cách xử lý lỗi — cho cả Go và Python, vì repo này đa ngôn
ngữ (Go cho accounts-service/ledger-service/settlement-engine, Python cho
notification-service và toàn bộ `tests/`).

Nằm ngoài phạm vi của đợt này (có thể trở thành spec/plan riêng sau):
- Cấu hình lint tooling (golangci-lint, ruff/mypy config file)
- Enforcement ở CI (chưa có `.github/workflows` nào)
- Checklist PR / code review

## Vị trí & Cấu trúc file

File mới: `docs/CODING_STANDARDS.md`, với hai mục cấp cao nhất —
**Đặt tên & Định dạng** và **Cách xử lý lỗi** — mỗi mục chia thành phần
Go và phần Python (hai ngôn ngữ có idiom khác nhau, nên rule không được
gộp chung).

Mỗi rule được viết theo cặp ✅ **Nên** / ❌ **Không nên** kèm ví dụ code
lấy từ chính domain của dự án (ledger entry, account ID, số tiền, risk
scoring) thay vì ví dụ chung chung kiểu `foo`/`bar` — người đọc mục tiêu
đang học lại cả hai ngôn ngữ, nên ví dụ cụ thể, bám sát domain quan trọng
hơn là văn phong style-guide trừu tượng.

`CLAUDE.md` sẽ có thêm một dòng mới trong mục **Stack**, trỏ tới
`docs/CODING_STANDARDS.md`, theo đúng pattern đã dùng để trỏ tới
`docs/PROJECT_CHARTER.md`.

## Nội dung: Đặt tên & Định dạng

**Go:**
- Tên package: ngắn, chữ thường, không gạch dưới (`ledger`, không phải
  `ledger_service`)
- Định dạng qua `gofmt`/`goimports` là bắt buộc, không thương lượng —
  đây là baseline cố định của Go, không phải một lựa chọn lint tùy chọn
- Import được nhóm thành ba khối, ngăn cách bằng dòng trống: standard
  library → third-party → internal package (khớp với pattern đã dùng
  trong code của plan ledger-service-mvp)
- Từ viết tắt giữ nhất quán cách viết hoa: `AccountID`/`TransactionID`,
  không bao giờ viết `AccountId`/`TransactionId`
- Biến sentinel error luôn có tiền tố `Err`: `ErrUnbalancedTransaction`,
  không phải `UnbalancedTransactionError`
- Hàm test: `TestXxx`, file test: `_test.go`

**Python:**
- `snake_case` cho hàm/biến/tên file, `PascalCase` cho class
- Định dạng qua `black`, giới hạn dòng 88 ký tự
- Import được nhóm thành ba khối giống Go: standard library →
  third-party → local module
- Khuyến khích dùng type hint cho function signature, đặc biệt ở bất kỳ
  đâu liên quan tới tiền hoặc risk scoring (`def score_transaction(amount:
  int) -> RiskScore:`)

## Nội dung: Cách xử lý lỗi

**Go:**
- Trả về `error` là giá trị trả về cuối cùng; không dùng `panic` cho các
  điều kiện lỗi có thể lường trước (input sai, lỗi DB) — `panic` chỉ dành
  cho lỗi lập trình không thể phục hồi
- Các lỗi domain đã biết, mà caller có thể kiểm tra được, dùng sentinel
  error (`errors.New`), khai báo gần kiểu domain mà nó thuộc về — đúng
  pattern đã thiết lập với `ErrUnbalancedTransaction`, `ErrInvalidAmount`,
  `ErrInvalidDirection`, `ErrNoEntries` trong plan ledger-service
- Bọc (wrap) lỗi khi đi qua layer bằng `fmt.Errorf("...: %w", err)`,
  không bao giờ dùng `%v` — `%w` giữ lại lỗi gốc để caller vẫn có thể
  dùng `errors.Is`/`errors.As`
- Không bao giờ so sánh lỗi bằng string (`err.Error() == "..."`); luôn
  dùng `errors.Is`
- Không bao giờ âm thầm bỏ qua lỗi (`_ = err`) mà không có comment giải
  thích rõ ràng vì sao
- Ở ranh giới HTTP (`api/handlers.go`), map lỗi domain sang HTTP status
  code một cách tường minh qua `errors.Is` (như trong handler
  `CreateTransaction` của plan, map `ErrUnbalancedTransaction` → 422);
  không bao giờ để message lỗi nội bộ lộ ra ngoài response

**Python:**
- Dùng exception cho các điều kiện thực sự ngoại lệ, không dùng return
  code/`None` như tín hiệu báo lỗi
- Lỗi đặc thù theo domain có class exception riêng (ví dụ
  `class RiskScoringError(Exception)`, một khi code
  notification-service/tests tồn tại), thay vì raise một exception dựng
  sẵn chung chung khi một exception đặc thù domain sẽ rõ nghĩa hơn
- Khi wrap/re-raise, dùng `raise NewError(...) from err` để giữ lại
  traceback gốc — tương đương với `%w` của Go bên phía Python
- Chỉ catch những exception type mà bạn thực sự xử lý được có ý nghĩa;
  tránh `except Exception:` trần trụi trừ khi ở một ranh giới ngoài cùng
  tường minh (ví dụ tooling triage lỗi CI) kèm comment giải thích vì sao

## Testing

Không áp dụng — đây là thay đổi chỉ liên quan tài liệu (không có code
thực thi để test). Việc xác minh là tự review lại tài liệu đã viết để
đảm bảo nhất quán với các ví dụ đã có sẵn trong
`docs/superpowers/plans/2026-07-29-ledger-service-mvp.md`.
