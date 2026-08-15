# Thiết kế: Subagent QA nghiệp vụ (`business-qa`)

**Ngày:** 2026-08-11
**Trạng thái:** Đã duyệt để triển khai

## Mục đích

SettleGuard đang bước vào giai đoạn code phần lõi nhất — `settlement-engine`
(risk scoring, settlement batching) — nơi phần lớn logic là quy tắc nghiệp
vụ (business rule) chứ không phải boilerplate. Hiện tại việc kiểm tra tính
đúng nghiệp vụ chỉ dựa vào review thủ công hoặc skill `code-review` chung
(tập trung vào bug/simplification, không có khái niệm "quy tắc nghiệp vụ
của SettleGuard").

Spec này định nghĩa một subagent mới, `business-qa`, chuyên soát tính đúng
nghiệp vụ sau khi một phần logic nghiệp vụ vừa được code xong, cộng với một
file quy tắc nghiệp vụ (`docs/BUSINESS_RULES.md`) làm nguồn sự thật để
subagent đối chiếu.

Nằm ngoài phạm vi đợt này:

- Tự động hoá theo lịch/cron (agent chỉ gọi thủ công, không chạy nền liên tục)
- Tự động sửa code khi phát hiện lỗi (agent chỉ báo cáo)
- CI integration (repo chưa có `.github/workflows`)

## Kiến trúc & quyền hạn

File mới: `.claude/agents/business-qa.md`, gọi qua `Agent` tool
(`subagent_type: business-qa`).

- **Tool được cấp**: `Read`, `Grep`, `Glob`, `Bash`, `ReportFindings`.
  **Không** có `Edit`/`Write` — đây là agent QA thuần, không tự sửa code
  đang được soát, tránh vừa là người viết vừa là người tự chấm điểm.
- **Model**: kế thừa model của phiên gọi nó (không override) — việc đối
  chiếu quy tắc nghiệp vụ cần suy luận, không phù hợp model rẻ.
- **Input**: agent cha (Claude chính hoặc người dùng) phải brief rõ phạm
  vi — ví dụ "vừa code xong risk scoring rules trong settlement-engine,
  soát theo BUSINESS_RULES.md" — agent không tự đoán ngữ cảnh vì chạy độc
  lập, không thấy lịch sử hội thoại trước đó.

## Quy trình QA — 2 tầng

**Tầng 1 — luôn chạy (đọc tĩnh, không cần hạ tầng):**

1. Xác định phạm vi: đọc diff hiện tại của branch (so với `service/ledger-service`,
   main branch của repo) hoặc đọc trọn service nếu được brief rõ "soát
   toàn bộ service X".
2. Đọc `docs/BUSINESS_RULES.md` để nắm danh sách rule đang có.
3. Đối chiếu từng đoạn logic nghiệp vụ trong diff với checklist — tìm chỗ
   code lệch quy tắc (ví dụ: quên check idempotency, quyết định `Hold`
   dùng AND thay vì OR, thiếu balance-check double-entry).
4. Nếu phát hiện logic nghiệp vụ **mới** không có trong checklist → tạo
   finding loại `missing-rule-doc` (nhắc bổ sung vào `BUSINESS_RULES.md`;
   agent không tự sửa file đó, theo đúng nguyên tắc report-only).

**Tầng 2 — chạy có điều kiện**, chỉ khi diff đụng tới event/contract
(publish/consume NATS, đổi payload struct, đổi API request/response
nghiệp vụ):

1. Tìm kịch bản verification có sẵn trong
   `docs/superpowers/plans/*.md` của service liên quan (các plan hiện tại,
   ví dụ `2026-08-10-settlement-engine-mvp.md`, đã có sẵn mục "Manual"
   verification bằng curl/DB check). Dùng làm kịch bản gốc nếu có; nếu
   không, tự soạn kịch bản tối thiểu từ event contract đọc được trong code.
2. `docker compose -f infra/docker/docker-compose.yml up -d` các Postgres
   và NATS container liên quan, `go run ./cmd/server` cho (các) service
   liên quan chạy nền.
3. Gọi API thật (`curl`) để tạo dữ liệu, chờ event lan truyền, kiểm tra
   state cuối (DB rows / response) đúng như hợp đồng đã khai báo.
4. Đối chiếu struct payload event giữa service publish và service consume
   (grep định nghĩa struct ở cả hai phía — ví dụ
   `internal/ledgerevent/payload.go` ở ledger-service so với bản mirror
   ở accounts-service/settlement-engine) — phát hiện lệch field/type/tên.
5. Dọn dẹp: dừng các process `go run` mà agent tự khởi chạy. **Không**
   `docker compose down` — giữ nguyên container, tránh phá môi trường dev
   người dùng đang dùng cho việc khác.

Nếu hạ tầng lỗi (Docker không chạy, port bị chiếm...) → agent báo finding
riêng mức info/thấp ("không chạy được tầng 2, lý do: ..."), không tính là
lỗi nghiệp vụ.

## `docs/BUSINESS_RULES.md`

File mới, cùng nhóm tra cứu với `docs/WORDING.md` (tra khái niệm) và
`docs/SYNTAX_REFERENCE.md` (tra cú pháp) — file này tra **invariant/quy
tắc nghiệp vụ bắt buộc đúng**.

Tổ chức theo service, mỗi rule có ID ngắn (`<SERVICE>-<số>`) để agent trích
dẫn trong finding. Mỗi mục gồm ba phần: **Rule** (phát biểu ngắn), **Vì
sao** (hậu quả nếu sai), **Ở đâu trong code** (file/package liên quan).

Nội dung khởi tạo, rút từ `CLAUDE.md` và các spec/plan đã có (không thêm
gì chưa được quyết định trong dự án):

- **LEDGER-01**: Double-entry — tổng debit phải bằng tổng credit trong 1
  transaction.
- **LEDGER-02**: Ledger append-only — không update/delete bút toán đã ghi.
- **ACCOUNTS-01**: `balance = Σcredit − Σdebit`, tính từ ledger entry.
- **ACCOUNTS-02**: Idempotent theo `transaction_id` khi consume
  `ledger.entry-recorded` (bảng dedup `processed_ledger_transactions`).
- **ACCOUNTS-03**: Account thuộc client đang `suspended` → chặn tạo mới.
- **SETTLEMENT-01**: Quyết định Hold là **OR** giữa 3 rule
  (velocity/mismatch/blocklist) — không phải AND, không phụ thuộc `Score`.
- **SETTLEMENT-02**: Đếm velocity phải loại trừ chính transaction đang
  chấm điểm (query trước khi persist).
- **SETTLEMENT-03**: `Score` cộng dồn theo trọng số cố định, cap tại 100.
- **SETTLEMENT-04**: Batch chỉ gom transaction `pending_settlement`; loại
  `held` và đã `settled`.
- **CROSS-01**: Mọi consumer của event NATS phải idempotent (do broker
  giao at-least-once).
- **CROSS-02**: Ghi DB + chuẩn bị publish event phải nằm cùng 1
  transaction (outbox pattern) — không tách rời.

File cần cập nhật thủ công khi có rule nghiệp vụ mới; `business-qa` chỉ
*nhắc* qua finding `missing-rule-doc`, không tự sửa file này.

## Định dạng báo cáo

Dùng `ReportFindings` (cùng cơ chế với skill `code-review`). Mỗi finding:

- `category`: một trong `business-rule-violation` | `contract-mismatch` |
  `e2e-failure` | `missing-rule-doc`
- `file`/`line`: vị trí trong code nếu có; lỗi e2e không gắn được với 1
  dòng cụ thể thì để trống, mô tả rõ trong `failure_scenario`
- `summary`, `failure_scenario`: viết bằng **tiếng Việt**, trích rule ID
  liên quan khi có (ví dụ: "Vi phạm SETTLEMENT-01: ...")
- Xếp theo mức nghiêm trọng: sai lệch có thể gây mất tiền/mất dữ liệu
  (double-entry, idempotency) > contract mismatch > lỗi hạ tầng/thiếu doc

Nếu không có finding nào, agent báo ngắn gọn "không phát hiện sai lệch
nghiệp vụ" thay vì gọi `ReportFindings` với mảng rỗng cho có.

## Testing

Không áp dụng cho việc viết agent definition + `BUSINESS_RULES.md` (tài
liệu + config, không có code thực thi). Xác minh bằng cách gọi thử
`business-qa` trên một thay đổi nghiệp vụ thật đầu tiên (khi
`settlement-engine`'s Task 7 — risk scoring — hoàn thành) và kiểm tra thủ
công: agent có bắt được ít nhất 1 vi phạm cố tình chèn vào (ví dụ đổi OR
thành AND ở quyết định Hold) không, trước khi tin tưởng dùng thường xuyên.
