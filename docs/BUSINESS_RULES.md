# Quy tắc nghiệp vụ (Business Rules)

Checklist các invariant nghiệp vụ bắt buộc đúng trong SettleGuard, tra
theo ID. Dùng làm nguồn sự thật cho subagent `business-qa` (xem
`.claude/agents/business-qa.md`) khi soát code, và cho bất kỳ ai review
logic nghiệp vụ thủ công.

Cùng nhóm với `docs/WORDING.md` (tra khái niệm) và
`docs/SYNTAX_REFERENCE.md` (tra cú pháp) — file này tra **quy tắc phải
đúng**, không phải khái niệm hay cú pháp.

Khi code thêm một invariant nghiệp vụ mới (rule risk-scoring mới, ràng
buộc dữ liệu mới...) mà chưa có trong danh sách dưới, hãy thêm rule mới
vào đây — đừng để nó chỉ tồn tại ngầm trong code.

## ledger-service

- **LEDGER-01** — Double-entry: trong một transaction, tổng số tiền các
  bút toán debit phải bằng tổng các bút toán credit.
  **Vì sao:** đây là cơ chế tự phát hiện sai lệch cốt lõi của kế toán
  double-entry; sai thì toàn bộ số liệu downstream (balance, settlement)
  không còn tin cậy được.
  **Ở đâu:** `services/ledger-service/internal/ledger` (validate trước
  khi ghi transaction).

- **LEDGER-02** — Ledger append-only: không update/delete một bút toán đã
  ghi.
  **Vì sao:** ledger là nguồn sự thật (source of truth) mọi service khác
  dựa vào; sửa/xoá ngầm phá vỡ khả năng audit và replay.
  **Ở đâu:** `services/ledger-service/internal/ledger` (không có
  endpoint/method update/delete cho ledger entry).

## accounts-service

- **ACCOUNTS-01** — `balance = Σcredit − Σdebit`, tính từ các ledger entry
  của account đó.
  **Vì sao:** balance không phải trường lưu trực tiếp mà phải luôn suy ra
  đúng từ ledger; lệch công thức là lệch số dư nghĩa vụ hiển thị cho
  client.
  **Ở đâu:** `services/accounts-service/internal/account`
  (`ApplyLedgerTransaction` hoặc tương đương).

- **ACCOUNTS-02** — Idempotent theo `transaction_id` khi consume
  `ledger.entry-recorded`: xử lý cùng một event 2 lần không được cộng dồn
  balance 2 lần.
  **Vì sao:** NATS JetStream giao event theo kiểu at-least-once, có thể
  redeliver; không idempotent → balance tăng/giảm sai khi bị gửi trùng.
  **Ở đâu:** bảng dedup `processed_ledger_transactions`,
  `services/accounts-service/internal/account/account_repository.go`.

- **ACCOUNTS-03** — Account thuộc một `ClientBusiness` đang ở trạng thái
  `suspended` thì không được tạo account mới cho client đó.
  **Vì sao:** chặn hoạt động nghiệp vụ mới phát sinh dưới một client đã bị
  đình chỉ.
  **Ở đâu:** `services/accounts-service/internal/account` (handler tạo
  account).

## settlement-engine

- **SETTLEMENT-01** — Quyết định `Hold` là **OR** giữa 3 rule (velocity
  limit, mismatch threshold, blocklist) — hold nếu bất kỳ rule nào
  trigger, không phải AND của cả 3. `Score` là trường thông tin/audit
  riêng, không quyết định `Decision`.
  **Vì sao:** dùng AND sẽ để lọt giao dịch chỉ vi phạm 1 rule nhưng vẫn
  rủi ro thật — đây là cơ chế phòng gian lận cốt lõi của cả hệ thống.
  **Ở đâu:** `services/settlement-engine/internal/risk` (`Scorer.Score`).

- **SETTLEMENT-02** — Đếm velocity (`CountRecentTransactions`) phải thực
  hiện *trước khi* persist transaction đang chấm điểm, và phải loại trừ
  chính nó khỏi số đếm.
  **Vì sao:** nếu tự đếm chính mình, một giao dịch đơn lẻ có thể tự kích
  hoạt velocity limit một cách sai lệch (off-by-one).
  **Ở đâu:** `services/settlement-engine/internal/settlement`
  (`TransactionRepository.CountRecentTransactions`).

- **SETTLEMENT-03** — `Score` cộng dồn theo trọng số cố định của từng rule
  trigger, cap tối đa tại 100.
  **Vì sao:** `Score` dùng để audit/so sánh mức độ rủi ro tương đối giữa
  các giao dịch; không cap sẽ làm số liệu không so sánh được, gây hiểu
  nhầm mức độ nghiêm trọng.
  **Ở đâu:** `services/settlement-engine/internal/risk` (`Scorer.Score`).

- **SETTLEMENT-04** — Một settlement batch chỉ được gom các transaction ở
  trạng thái `pending_settlement`; loại trừ `held` và những transaction
  đã `settled` từ trước.
  **Vì sao:** gom nhầm transaction `held` vào batch settlement là vô hiệu
  hoá toàn bộ risk-scoring; gom lại transaction đã `settled` là tính
  trùng.
  **Ở đâu:** `services/settlement-engine/internal/settlement`
  (`SettlementRepository.RunBatch`).

## Quy tắc xuyên suốt (cross-cutting)

- **CROSS-01** — Mọi consumer của NATS JetStream event phải idempotent —
  xử lý cùng một event 2 lần cho kết quả giống hệt xử lý 1 lần.
  **Vì sao:** broker giao event theo kiểu at-least-once (có thể redeliver
  khi lỗi mạng/consumer crash trước khi ack); consumer không tự chịu được
  trùng lặp sẽ tính sai dữ liệu tài chính.
  **Ở đâu:** mọi package `internal/consumer` + bảng dedup
  `processed_*_transactions` ở từng service.

- **CROSS-02** — Ghi dữ liệu vào DB và chuẩn bị publish event phải nằm
  trong cùng một DB transaction (outbox pattern) — không được tách rời
  thành hai bước độc lập.
  **Vì sao:** nếu ghi DB xong rồi crash trước khi gửi event, downstream
  service sẽ không bao giờ biết về thay đổi đó — outbox pattern đảm bảo
  hai việc này atomic với nhau.
  **Ở đâu:** `internal/outbox` (bảng `outbox_events` + `Relay`) ở từng
  service.
