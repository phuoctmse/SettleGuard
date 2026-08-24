# tests/ suite — Thiết kế

Ngày: 2026-08-24
Trạng thái: Nháp — cần review trước khi chuyển thành implementation plan

## 1. Bối cảnh & Phạm vi

Cả 4 service backend (`ledger-service`, `accounts-service`,
`settlement-engine`, `notification-service`) và `mobile-app` giờ đều đã có
MVP chạy được (xem `CLAUDE.md` § Project Status). Thứ duy nhất trong
Testing Layout của `CLAUDE.md` chưa tồn tại là chính thư mục `tests/` —
`tests/api`, `tests/security`, `tests/perf`, `tests/ai-tools`, và Appium
E2E cho mobile đều mới chỉ là mô tả, chưa có code.

Spec này rà lại từng mảng, đối chiếu với những gì đã thực sự tồn tại trong
repo (không phải mô tả lý tưởng trong `CLAUDE.md`), để quyết định mảng nào
làm được ngay và mảng nào đang bị chặn bởi một prerequisite chưa có.

### Rà soát từng mảng

| Mảng | Mô tả trong CLAUDE.md | Trạng thái thực tế |
|---|---|---|
| `tests/api` | Test API/contract, chạy dựa trên OpenAPI spec của từng service | `docs/openapi.yaml` **tồn tại nhưng rỗng** (0 dòng) — chưa có OpenAPI spec thật nào để dựa vào |
| `tests/security` | Fraud-bypass, auth boundary checks | Fraud-bypass **test được ngay** (business rule đã có sẵn, xem bảng dưới). Auth boundary checks **không test được** — không service nào có auth (đã hoãn tường minh ở mọi service, xem accounts-service spec) |
| `tests/perf` | Load/perf cho real-time scoring + batch settlement throughput | Test được ngay — endpoint và cấu hình rule (`SETTLEMENT_VELOCITY_LIMIT` etc.) đã ổn định |
| `tests/ai-tools` | Sinh test case từ OpenAPI spec + triage lỗi CI | **Chặn kép**: chưa có OpenAPI spec (như trên) *và* chưa có CI nào tồn tại (`.github/` không có, xác nhận lại từ spec coding-standards: "chưa có `.github/workflows` nào") — không có gì để sinh test hay để triage |
| Mobile Appium E2E | Điều khiển app thật qua Appium | **Chặn**: `mobile-app/src` chưa có `testID`/`accessibilityLabel` nào — Appium cần selector ổn định để tìm element, không có thì không viết được test nào đáng tin cậy |

### Quyết định phạm vi

Chia làm 2 phần rõ ràng thay vì cố làm cả 5 mảng cùng lúc:

- **Trong phạm vi plan này**: `tests/api`, `tests/security` (chỉ phần
  fraud-bypass), `tests/perf`. Cả 3 đều không bị chặn bởi prerequisite
  nào ngoài phạm vi của chính plan này.
- **Để lại cho việc khác** (nêu rõ lý do, không giả vờ làm cho có):
  `tests/ai-tools` (cần viết OpenAPI spec thật + dựng CI trước — cả hai
  đều là công việc lớn, xứng đáng có plan riêng), auth boundary checks
  trong `tests/security` (chờ quyết định chung về auth), Appium E2E cho
  mobile (chờ `mobile-app` được gắn `testID` — việc đó thuộc về
  `mobile-app`, không phải `tests/`).

Coding mode: Claude code trực tiếp (`tests/*` nằm trong phạm vi "Claude
codes autonomously" của Working Style).

## 2. Kiến trúc & Tech Stack

- **Python + pytest**, khớp `CLAUDE.md` § Stack ("Python (notification-service,
  toàn bộ `tests/`, AI tooling)") — không dùng Go dù phần lớn service là
  Go, để giữ đúng quy ước đã chọn từ đầu dự án (test automation là lãnh
  địa Python riêng, tách khỏi ngôn ngữ triển khai của từng service).
- **`httpx`** làm HTTP client (không dùng `requests` — `httpx` hỗ trợ cả
  sync lẫn async, và team Python duy nhất khác trong repo
  (`notification-service`) không có tiền lệ nào ép buộc `requests`, nên
  chọn thư viện hiện đại hơn không có lý do phải tránh).
- **`tests/api` không sinh test từ OpenAPI spec** (spec rỗng) — viết test
  hợp đồng trực tiếp, tham chiếu đúng bảng `## API` đã ghi trong README
  của từng service. Khi OpenAPI spec thật được viết (việc khác), có thể
  bổ sung lớp sinh-test-tự-động sau — không chặn việc viết contract test
  thủ công trước.
- **`tests/perf` dùng Locust** — công cụ load-test chuẩn của hệ sinh thái
  Python, so với tự viết vòng lặp `asyncio.gather` thô thì Locust cho sẵn
  ramp-up, phân bố user ảo, và báo cáo — không có lý do tự chế lại.
- **Không có single-command "start toàn bộ stack".**
  `infra/docker/docker-compose.yml` hiện chỉ chứa Postgres + NATS cho mỗi
  service, không chứa chính các service Go/Python (chúng chạy bằng
  `go run`/`python main.py` thủ công theo từng README). `tests/api` và
  `tests/perf` đều cần cả 4 service đang chạy — thay vì tạo thêm tầng
  orchestration mới (docker-compose cho app code, Makefile, script khởi
  động...), decision ở đây là: **conftest.py health-check từng service
  trước khi chạy test, `pytest.skip` kèm thông báo rõ ràng nếu service nào
  không reachable**, thay vì để test fail với lỗi connection-refused khó
  hiểu. Việc dựng orchestration tự động (docker-compose cho app code) là
  việc khác, thuộc `infra/` — ngoài ownership của Claude theo Working
  Style (`infra/*` do user tự quản).
- Layout: `tests/api/{conftest.py,clients/,test_*.py}`,
  `tests/security/{conftest.py,test_*.py}` (tái dùng `tests/api/clients`
  qua `sys.path`/package import — xem chi tiết layout ở mục 5),
  `tests/perf/locustfile_*.py`.

## 3. `tests/api` — Phạm vi test

Không test lại logic nội bộ từng service (đã có unit/integration test
riêng trong từng service rồi — `go test ./...`, `pytest` per-service).
`tests/api` test **hợp đồng giữa các service** và **luồng xuyên suốt**,
đúng vai trò "cross-service API/contract tests" mà CLAUDE.md mô tả:

1. **Per-service contract smoke test** — với mỗi service, gọi đúng những
   endpoint đã liệt kê trong README `## API`, xác nhận status code và
   shape response cơ bản đúng như tài liệu (vd `POST /clients` với body
   rỗng → `400`; `GET /accounts/{id}` với id không tồn tại → `404`).
   Không lặp lại toàn bộ edge case đã có unit test — chỉ xác nhận "đúng
   như README nói", vì README chính là hợp đồng client (mobile-app) đang
   dựa vào.
2. **Luồng end-to-end xuyên 4 service** — đúng tiêu chí thành công v1 của
   charter: "Một giao dịch chạy trọn vẹn từ đầu đến cuối: được ghi vào
   ledger → được chấm điểm rủi ro theo thời gian thực → được đưa vào đợt
   tất toán theo lô tiếp theo (hoặc bị giữ lại) → gửi thông báo". Cụ thể:
   - Tạo client + account (accounts-service) → post transaction
     (ledger-service) → poll `settlement-engine`'s `GET /transactions/{id}`
     tới khi `status != pending` ban đầu (risk-scored) → nếu `pending_settlement`,
     đợi tới batch tiếp theo rồi xác nhận `GET /settlements` chứa nó; nếu
     `held`, gọi `POST /transactions/{id}/approve` rồi xác nhận đi tiếp vào
     batch → xác nhận `notification-service`'s `GET /notifications` có
     đúng bản ghi tương ứng.
   - Đây là test giá trị cao nhất trong toàn bộ suite — lần đầu tiên có
     một test chạm vào cả 4 service cùng lúc qua NATS thật, không mock ở
     đâu cả.
3. Không test giao diện mobile-app (đó là việc của Appium E2E, đang bị
   chặn — xem mục 1).

## 4. `tests/security` — Phạm vi test (chỉ fraud-bypass)

Mỗi test map trực tiếp tới một `SETTLEMENT-0x` đã ghi trong
`docs/BUSINESS_RULES.md`, cố tình thử "lách luật" và xác nhận hệ thống
chặn đúng:

- **Velocity limit bypass** — gửi nhiều hơn `SETTLEMENT_VELOCITY_LIMIT`
  giao dịch cho cùng account trong cửa sổ thời gian, xác nhận giao dịch
  vượt ngưỡng bị `held` (SETTLEMENT-01/02).
- **Mismatch threshold bypass** — gửi giao dịch vượt
  `SETTLEMENT_MISMATCH_THRESHOLD`, xác nhận `held`.
- **Blocklist bypass** — thêm account vào `blocklist`, xác nhận mọi giao
  dịch chạm account đó đều `held` bất kể amount/velocity.
- **Double-resolve race** — gọi `POST /transactions/{id}/approve` hai lần
  liên tiếp (hoặc approve rồi reject), xác nhận lần thứ hai trả `409`,
  không có cách nào khiến một transaction vừa `pending_settlement` vừa
  `rejected` (SETTLEMENT-05).
- **Held transaction không lọt vào batch khi chưa resolve** — xác nhận
  `GET /settlements` sau một batch run không chứa transaction đang
  `held` (SETTLEMENT-04).

Không có test nào cho "auth boundary" — không có auth để test (xem mục 1).
`tests/security/README.md` (viết ở task cuối) sẽ ghi rõ điều này thay vì
để trống gây hiểu lầm là quên làm.

## 5. Layout & chia sẻ code giữa `tests/api` và `tests/security`

```
tests/
  api/
    conftest.py          # fixture: base URLs từng service, health-check + skip
    clients/
      __init__.py
      accounts.py         # AccountsClient — wrap httpx theo endpoint accounts-service
      ledger.py
      settlement.py
      notifications.py
    test_accounts_contract.py
    test_ledger_contract.py
    test_settlement_contract.py
    test_notification_contract.py
    test_end_to_end_flow.py
  security/
    conftest.py            # import lại fixture từ tests/api/conftest.py
    test_fraud_bypass.py
  perf/
    locustfile_settlement_scoring.py
  pyproject.toml           # pytest config dùng chung cho tests/api + tests/security
  requirements.txt
```

`tests/security` tái dùng `tests/api/clients/*` (import trực tiếp qua
`sys.path` được `pyproject.toml`/`conftest.py` cấu hình, giống cách
`internal/` được import trong `notification-service`) — tránh viết lại
một bộ HTTP client thứ hai chỉ để đổi mục đích test.

`tests/perf` **không** tái dùng `tests/api/clients` — Locust có mô hình
request riêng (`HttpUser`/`task`), ép nó qua lớp client viết cho pytest
sẽ chỉ thêm một tầng gián tiếp không cần thiết.

## 6. Testing (cho chính test suite này)

Nghịch lý thường gặp: ai test cái test? Ở đây không cần — `tests/api` và
`tests/security` tự thân đã là test (chạy được = đã tự xác nhận đúng, vì
chúng chạm hệ thống thật, không phải logic nội bộ cần unit test riêng).
`conftest.py`'s health-check/skip logic là phần logic thật duy nhất đáng
lo sai — verify thủ công bằng cách chạy `pytest tests/api` khi cố tình tắt
một service, xác nhận thấy `SKIPPED` với thông báo rõ ràng thay vì
`ERROR`.

## 7. Tiêu chí thành công cho phạm vi plan này

- `pytest tests/api` pass toàn bộ khi cả 4 service + Postgres + NATS đang
  chạy (theo đúng lệnh khởi động ghi trong README mỗi service).
- `pytest tests/api` **skip rõ ràng** (không fail bí ẩn) khi thiếu bất kỳ
  service nào.
- Test luồng end-to-end xuyên 4 service pass được cả nhánh `pending_settlement`
  lẫn nhánh `held` → `approve` → settle.
- `pytest tests/security` pass toàn bộ 5 test fraud-bypass.
- `locust -f tests/perf/locustfile_settlement_scoring.py` chạy được, xác
  nhận thủ công báo cáo hợp lý (không crash, ghi nhận latency/throughput).

## 8. Để lại cho việc khác (không nằm trong phạm vi plan này)

- **`tests/ai-tools`** — cần OpenAPI spec thật (viết `docs/openapi.yaml`
  cho cả 4 service, hiện đang rỗng) và cần CI tồn tại trước (chưa có
  `.github/workflows` nào) — cả hai đủ lớn để thành plan riêng.
- **Auth boundary checks** trong `tests/security` — chờ quyết định chung
  về auth (chưa service nào có).
- **Mobile Appium E2E** — chờ `mobile-app` được gắn `testID`/
  `accessibilityLabel` (việc thuộc về `mobile-app`, không phải `tests/`).
- CI wiring (chạy `tests/api`/`tests/security` tự động) — phụ thuộc vào
  việc dựng CI nói chung, ngoài phạm vi.
