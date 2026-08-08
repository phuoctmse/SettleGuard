# Tài liệu tham khảo Syntax: Python

Cheatsheet tra cứu nhanh cú pháp Python, ví dụ bám theo domain của
SettleGuard (ledger entry, account, risk score...) thay vì `foo`/`bar`.
Không phải coding standard (xem `docs/CODING_STANDARDS.md` cho quy ước
style/lỗi), đây chỉ là "tra syntax quên thì mở ra xem". Bản Go:
`docs/SYNTAX_REFERENCE_GO.md`.

## 1. Biến & kiểu dữ liệu — dynamic typing

```python
amount = 500          # không cần khai báo kiểu, kiểu gắn với GIÁ TRỊ chứ không phải biến
reason = "invoice"
is_valid = True
account_id = None     # None ~ null/nil
```

Không có `var`/`const` như Go. Convention: hằng số viết `UPPER_SNAKE_CASE`
(không có enforcement ở compiler, chỉ là quy ước — xem
`docs/CODING_STANDARDS.md`).

## 2. Type hint (khuyến khích, không bắt buộc)

```python
def score_transaction(amount: int) -> RiskScore:
    ...

account_id: str = "abc-123"
```

Type hint **không được Python runtime enforce** — chỉ là gợi ý cho người
đọc và cho type checker (`mypy`). Khác hẳn Go, nơi kiểu luôn được compiler
kiểm tra bắt buộc.

## 3. Cấu trúc dữ liệu

| Kiểu | Ví dụ | Mutable? |
|---|---|---|
| `list` | `[1, 2, 3]` | Có |
| `tuple` | `(1, 2, 3)` | Không |
| `dict` | `{"amount": 500, "reason": "invoice"}` | Có |
| `set` | `{1, 2, 3}` | Có |

```python
entries = []
entries.append(entry)          # thêm vào cuối, SỬA TRỰC TIẾP list (khác Go — không cần gán lại)

scores = {"tx-1": 0.2, "tx-2": 0.9}
score = scores.get("tx-1")      # trả None nếu không có key (không raise lỗi)
score = scores["tx-1"]          # raise KeyError nếu không có key
```

## 4. Hàm

```python
def score_transaction(amount, threshold=100):  # threshold có default value
    return amount > threshold

def notify(*args, **kwargs):  # *args: nhiều positional argument dạng tuple
    pass                       # **kwargs: nhiều keyword argument dạng dict

is_risky = lambda amount: amount > 1000  # hàm ẩn danh 1 dòng
```

## 5. Class

```python
class RiskScoringError(Exception):
    pass

class RiskScore:
    def __init__(self, amount: int, threshold: int = 100):
        self.amount = amount        # self = tương đương receiver của Go, PHẢI khai báo tường minh làm tham số đầu
        self.threshold = threshold

    def is_risky(self) -> bool:
        return self.amount > self.threshold

score = RiskScore(amount=500)
score.is_risky()
```

- `__init__` = constructor.
- Kế thừa: `class ChildScore(RiskScore):` — Python **có** kế thừa (khác Go).
- `PascalCase` cho tên class, `snake_case` cho method/biến.

## 6. Xử lý lỗi — exception, không phải return-error như Go

```python
try:
    score = score_transaction(amount)
except ValueError as e:
    raise RiskScoringError("invalid amount") from e  # "from e" giữ traceback gốc — tương đương %w của Go
finally:
    cleanup()  # luôn chạy, dù có lỗi hay không — giống defer của Go nhưng khai báo khác chỗ
```

Khác biệt cốt lõi so với Go: Python dùng **exception** cho lỗi (raise/catch),
Go dùng **giá trị `error` trả về** phải check tường minh. Không "quên check
lỗi" được ở Python theo cách của Go — nhưng ngược lại dễ quên `except` đúng
loại lỗi cần bắt (xem `docs/CODING_STANDARDS.md` — tránh `except Exception:`
trần trụi).

## 7. Control flow

```python
if amount <= 0:
    raise ValueError("amount must be positive")
elif amount > 1_000_000:
    raise ValueError("amount too large")
else:
    process(amount)

for entry in entries:          # for luôn là "for-each" trong Python
    print(entry)

for i in range(10):             # range(10) = 0..9
    print(i)

while retries < 3:
    retries += 1
```

Không có `switch` truyền thống (Python 3.10+ có `match`/`case` nhưng chưa
dùng trong repo này). Ternary có dạng khác Go: `x if condition else y`.

## 8. List/dict comprehension — không có tương đương trực tiếp trong Go

```python
risky_amounts = [e.amount for e in entries if e.amount > 1000]
scores_by_tx = {e.transaction_id: e.amount for e in entries}
```

## 9. Context manager (`with`) — giống `defer` nhưng theo khối, không theo hàm

```python
with open("data.csv") as f:
    data = f.read()
# f tự động được đóng khi ra khỏi khối with, kể cả khi có exception
```

## 10. pytest — test trong repo này dùng pytest, không dùng `unittest`

```python
def test_score_transaction_flags_high_amount():
    result = score_transaction(amount=2000, threshold=1000)
    assert result.is_risky is True

import pytest

@pytest.mark.parametrize("amount,expected", [
    (500, False),
    (2000, True),
])
def test_is_risky(amount, expected):
    assert score_transaction(amount).is_risky == expected
```

- File: `test_*.py` hoặc `*_test.py`, hàm: `def test_*():`.
- `assert` trần (không cần `assertEqual` như `unittest`) — pytest tự in ra
  giá trị 2 bên khi assert fail.
- Chạy: `pytest tests/path/test_file.py -v`, 1 test cụ thể:
  `pytest tests/path/test_file.py::test_is_risky -v`.
