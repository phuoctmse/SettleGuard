# Tài liệu tham khảo Syntax: Go

Cheatsheet tra cứu nhanh cú pháp Go, ví dụ bám theo domain của SettleGuard
(ledger entry, account, risk score...) thay vì `foo`/`bar`. Không phải
coding standard (xem `docs/CODING_STANDARDS.md` cho quy ước style/lỗi),
đây chỉ là "tra syntax quên thì mở ra xem". Bản Python: `docs/SYNTAX_REFERENCE_PYTHON.md`.

## 1. Package & Import

Mỗi file Go thuộc về một package (dòng đầu tiên). Import được nhóm 3 khối:
standard library → third-party → internal package (xem
`docs/CODING_STANDARDS.md`).

```go
package ledger

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/phuoctmse/settleguard/ledger-service/internal/db"
)
```

- Package `main` + hàm `func main()` = entrypoint chạy được (`cmd/server/main.go`).
- Package khác (`ledger`, `api`, `db`) = thư viện nội bộ, không tự chạy.
- Chữ cái đầu tên (hàm/type/biến) **viết hoa = export** (dùng được từ package
  khác), **viết thường = chỉ dùng nội bộ package đó**. Đây là toàn bộ cơ chế
  "public/private" của Go — không có từ khóa `public`/`private` riêng.

## 2. Khai báo biến

```go
var amount int64          // khai báo có kiểu, giá trị mặc định = 0 (zero value)
var reason string = "invoice"
accountID := uuid.New()   // := suy ra kiểu, chỉ dùng được trong hàm (không dùng ở top-level)
const Debit Direction = "debit" // const: giá trị cố định, biết tại compile-time
```

Mọi kiểu trong Go đều có **zero value** khi khai báo mà không gán —
`int`/`int64` → `0`, `string` → `""`, `bool` → `false`, con trỏ/slice/map/interface →
`nil`. Không có khái niệm "undefined" như JS.

## 3. Kiểu dữ liệu cơ bản

| Kiểu | Ví dụ | Ghi chú |
|---|---|---|
| `int`, `int64` | `Amount int64` | Amount tiền dùng `int64` (đơn vị nhỏ nhất, tránh float) |
| `float64` | ít dùng cho tiền | sai số làm tròn — không dùng cho amount |
| `string` | `Reason string` | immutable, UTF-8 |
| `bool` | `exists bool` | `true`/`false` |
| `[]byte` | dữ liệu nhị phân | |
| custom type | `type Direction string` | đặt tên riêng cho 1 kiểu nền, xem mục 5 |

## 4. Struct — kiểu dữ liệu giống class nhưng không có kế thừa

```go
type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	AccountID     uuid.UUID
	Direction     Direction
	Amount        int64
	Reason        string
	CreatedAt     time.Time
}

e := Entry{AccountID: accountID, Direction: Debit, Amount: 500, Reason: "invoice"}
```

Go không có `class`/`extends`. Thay vào đó struct + method gắn vào struct
qua **receiver**:

```go
type Repository struct {
	db *sql.DB
}

// (r *Repository) là receiver — giống "self"/"this" nhưng khai báo tường minh
func (r *Repository) ListByAccount(ctx context.Context, accountID uuid.UUID) ([]Entry, error) {
	// ...
}

repo := &Repository{db: conn}
repo.ListByAccount(ctx, someID) // gọi method
```

Receiver `*Repository` (con trỏ) vs `Repository` (giá trị): dùng con trỏ khi
method cần sửa field của struct, hoặc struct lớn (tránh copy). Mặc định
dùng con trỏ trừ khi có lý do cụ thể không cần.

## 5. Custom type & const nhóm (kiểu "enum" của Go)

Go không có `enum` thật. Pattern thay thế: custom type + hằng số:

```go
type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)
```

`Direction` giờ là kiểu riêng (không lẫn với `string` thường), compiler sẽ
báo lỗi nếu bạn gán nhầm string tuỳ tiện vào field kiểu `Direction`.

## 6. Con trỏ (pointer)

```go
func Connect(dsn string) (*sql.DB, error) { ... } // *sql.DB = con trỏ tới sql.DB

conn, err := Connect(dsn)
```

`*T` = con trỏ tới giá trị kiểu T. `&x` = lấy địa chỉ của x. Không có
"pointer arithmetic" như C — chỉ dùng để tránh copy dữ liệu lớn hoặc để hàm
sửa được giá trị gốc.

## 7. Interface

```go
// Bất kỳ type nào có method Read(p []byte) (n int, err error) đều "là" io.Reader
// — không cần khai báo "implements" tường minh như Java/C#
type Reader interface {
	Read(p []byte) (n int, err error)
}
```

Interface trong Go được thoả mãn ngầm định (structural typing) — không có
từ khóa `implements`. Chưa dùng nhiều trong plan ledger-service hiện tại
nhưng sẽ gặp khi mock/test ở service khác.

## 8. Hàm nhiều giá trị trả về — pattern lỗi chuẩn của Go

```go
func Connect(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return conn, nil
}
```

Go không có exception cho luồng thường — `error` luôn là **giá trị trả về
cuối cùng**, và caller **luôn phải check** (`if err != nil`). Xem
`docs/CODING_STANDARDS.md` mục Error Handling để biết sentinel error,
`%w`, `errors.Is`/`errors.As`.

## 9. Control flow

```go
// if — không có dấu ngoặc quanh điều kiện, bắt buộc có {}
if amount <= 0 {
	return ErrInvalidAmount
}

// switch — không cần "break", tự dừng sau khi match 1 case
switch e.Direction {
case Debit:
	debitTotal += e.Amount
case Credit:
	creditTotal += e.Amount
default:
	return ErrInvalidDirection
}

// for — CHỈ có "for", không có while/do-while riêng
for i := 0; i < len(entries); i++ { ... }   // dạng C-style
for _, e := range entries { ... }            // dạng range (phổ biến nhất)
for condition { ... }                        // dạng "while"
for { ... }                                  // vòng lặp vô hạn (dùng break để thoát)
```

Không có ternary (`a ? b : c`) trong Go — luôn viết `if/else` đầy đủ.

## 10. Slice & Map

```go
entries := []Entry{}                 // slice rỗng
entries = append(entries, e)         // thêm phần tử — LƯU Ý: append trả về slice mới, phải gán lại

inserted := make([]Entry, len(entries)) // cấp sẵn độ dài len(entries)

m := map[string]int64{"invoice": 500} // map[KeyType]ValueType
v, ok := m["invoice"]                 // ok = false nếu key không tồn tại (không panic)
```

## 11. defer / panic / recover

```go
conn, err := db.Connect(dsn)
defer conn.Close() // chạy NGAY TRƯỚC KHI hàm hiện tại return, dù return ở đâu/lỗi gì
```

`defer` = lịch chạy 1 dòng lệnh lúc hàm kết thúc — dùng để đóng file/DB
connection/transaction rollback mà không bị quên. `panic`/`recover` là cơ
chế lỗi "không lường trước được" (bug lập trình), **không** dùng cho lỗi
nghiệp vụ thường gặp (input sai, DB lỗi) — những lỗi đó dùng `error` bình
thường (xem mục 8).

## 12. Test

```go
package ledger_test // "_test" package = test từ góc nhìn bên ngoài, chỉ gọi API export

func TestValidateBalanced(t *testing.T) {
	err := ValidateBalanced(entries)
	assert.NoError(t, err) // từ github.com/stretchr/testify/assert
}
```

- File `xxx_test.go`, hàm `func TestXxx(t *testing.T)`.
- `t.Run("tên case", func(t *testing.T) {...})` = subtest — cho phép 1 hàm
  test chạy nhiều case (table-driven test), thấy rõ trong
  `docs/superpowers/plans/2026-07-29-ledger-service-mvp.md` Task 2.
- Chạy: `go test ./... -v` (toàn bộ), `go test ./internal/ledger/... -run TestValidateBalanced -v` (1 test cụ thể).
