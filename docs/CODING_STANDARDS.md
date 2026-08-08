# Coding Standards

Quy ước đặt tên/định dạng và cách xử lý lỗi cho toàn repo SettleGuard, áp
dụng cho cả Go (`accounts-service`, `ledger-service`, `settlement-engine`)
và Python (`notification-service`, toàn bộ `tests/`). Tài liệu này tự thân
đầy đủ, không trỏ ra ngoài tới style guide bên ngoài.

Nguồn: `docs/superpowers/specs/2026-08-01-coding-standards-design.md`
("Đã duyệt để triển khai").

## Đặt tên & Định dạng

### Go

**Tên package: ngắn, chữ thường, không gạch dưới.**

✅ Nên
```go
package ledger
```
❌ Không nên
```go
package ledger_service
```

**Định dạng qua `gofmt`/`goimports` là bắt buộc, không thương lượng** —
đây là baseline cố định của Go, không phải một lựa chọn lint tùy chọn.

**Import nhóm thành ba khối, ngăn cách bằng dòng trống**: standard
library → third-party → internal package.

✅ Nên
```go
import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
)
```
❌ Không nên
```go
import (
	"github.com/phuoctmse/settleguard/ledger-service/internal/ledger"
	"context"
	"github.com/go-chi/chi/v5"
	"fmt"
)
```

**Từ viết tắt giữ nhất quán cách viết hoa.**

✅ Nên: `AccountID`, `TransactionID`
❌ Không nên: `AccountId`, `TransactionId`

**Biến sentinel error luôn có tiền tố `Err`.**

✅ Nên: `ErrUnbalancedTransaction`
❌ Không nên: `UnbalancedTransactionError`

**Hàm test: `TestXxx`, file test: `_test.go`.**

### Python

- `snake_case` cho hàm/biến/tên file, `PascalCase` cho class.
- Định dạng qua `black`, giới hạn dòng 88 ký tự.
- Import nhóm thành ba khối, giống Go: standard library → third-party →
  local module.
- Khuyến khích type hint cho function signature, đặc biệt ở bất kỳ đâu
  liên quan tới tiền hoặc risk scoring.

✅ Nên
```python
def score_transaction(amount: int) -> RiskScore:
    ...
```
❌ Không nên
```python
def score_transaction(amount):
    ...
```

## Cách xử lý lỗi

### Go

**Trả về `error` là giá trị trả về cuối cùng; không dùng `panic` cho các
điều kiện lỗi có thể lường trước** (input sai, lỗi DB) — `panic` chỉ dành
cho lỗi lập trình không thể phục hồi.

**Lỗi domain đã biết, mà caller có thể kiểm tra được, dùng sentinel error
(`errors.New`), khai báo gần kiểu domain mà nó thuộc về.**

✅ Nên
```go
var ErrUnbalancedTransaction = errors.New("ledger: unbalanced transaction")
var ErrInvalidAmount = errors.New("ledger: invalid amount")
var ErrInvalidDirection = errors.New("ledger: invalid direction")
var ErrNoEntries = errors.New("ledger: no entries")
```

**Bọc (wrap) lỗi khi đi qua layer bằng `fmt.Errorf("...: %w", err)`,
không bao giờ dùng `%v`** — `%w` giữ lại lỗi gốc để caller vẫn có thể
dùng `errors.Is`/`errors.As`.

✅ Nên
```go
return fmt.Errorf("insert transaction: %w", err)
```
❌ Không nên
```go
return fmt.Errorf("insert transaction: %v", err)
```

**Không bao giờ so sánh lỗi bằng string; luôn dùng `errors.Is`.**

✅ Nên
```go
if errors.Is(err, ledger.ErrUnbalancedTransaction) { ... }
```
❌ Không nên
```go
if err.Error() == "ledger: unbalanced transaction" { ... }
```

**Không bao giờ âm thầm bỏ qua lỗi (`_ = err`) mà không có comment giải
thích rõ ràng vì sao.**

**Ở ranh giới HTTP (`api/handlers.go`), map lỗi domain sang HTTP status
code một cách tường minh qua `errors.Is`**; không bao giờ để message lỗi
nội bộ lộ ra ngoài response.

✅ Nên
```go
if errors.Is(err, ledger.ErrUnbalancedTransaction) {
	writeError(w, http.StatusUnprocessableEntity, "unbalanced transaction")
	return
}
```
❌ Không nên
```go
if err != nil {
	writeError(w, http.StatusUnprocessableEntity, err.Error())
	return
}
```

### Python

- Dùng exception cho các điều kiện thực sự ngoại lệ, không dùng return
  code/`None` như tín hiệu báo lỗi.
- Lỗi đặc thù theo domain có class exception riêng (vd.
  `class RiskScoringError(Exception)`), thay vì raise một exception
  dựng sẵn chung chung khi một exception đặc thù domain sẽ rõ nghĩa hơn.
- Khi wrap/re-raise, dùng `raise NewError(...) from err` để giữ lại
  traceback gốc — tương đương `%w` của Go.

✅ Nên
```python
try:
    score = model.predict(features)
except ModelUnavailableError as err:
    raise RiskScoringError("risk model unavailable") from err
```
❌ Không nên
```python
try:
    score = model.predict(features)
except Exception:
    raise RiskScoringError("risk model unavailable")
```

- Chỉ catch những exception type mà bạn thực sự xử lý được có ý nghĩa;
  tránh `except Exception:` trần trụi trừ khi ở một ranh giới ngoài cùng
  tường minh (vd. tooling triage lỗi CI) kèm comment giải thích vì sao.
