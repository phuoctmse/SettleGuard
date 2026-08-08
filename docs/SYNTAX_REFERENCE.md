# Tài liệu tham khảo Syntax: Go & Python

Cheatsheet tra cứu nhanh cú pháp cơ bản, ví dụ bám theo domain của
SettleGuard (ledger entry, account, risk score...) thay vì `foo`/`bar`.
Không phải coding standard (xem `docs/CODING_STANDARDS.md` cho quy ước
style/lỗi), đây chỉ là "tra syntax quên thì mở ra xem" — không cần đọc hết
một lượt.

Tách theo từng ngôn ngữ:
- **Go** → `docs/SYNTAX_REFERENCE_GO.md`
- **Python** → `docs/SYNTAX_REFERENCE_PYTHON.md`

## Bảng so sánh nhanh Go ↔ Python

| Khái niệm | Go | Python |
|---|---|---|
| Khai báo biến | `x := 5` hoặc `var x int = 5` | `x = 5` |
| Kiểu | Static, compiler check | Dynamic, type hint chỉ là gợi ý |
| Lỗi | `error` trả về, check `if err != nil` | `raise`/`try...except` |
| "Class" | struct + method có receiver, không kế thừa | `class`, có kế thừa |
| Vòng lặp | chỉ `for` (3 dạng) | `for` (for-each), `while` |
| Null | `nil` | `None` |
| Cleanup theo hàm | `defer` | `try/finally` hoặc `with` |
| Test file | `_test.go`, hàm `TestXxx` | `test_*.py`, hàm `test_*` |
| Test framework | `testing` + `testify` | `pytest` |
| Package/module | `import "path/to/pkg"` | `import module` / `from pkg import x` |
