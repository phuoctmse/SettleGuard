# SettleGuard

[![CI](https://github.com/phuoctmse/SettleGuard/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/phuoctmse/SettleGuard/actions/workflows/ci.yml)

> Nền tảng B2B theo dõi và quyết toán **nghĩa vụ thanh toán** giữa các bên,
> có lớp **chấm điểm rủi ro** chặn giao dịch đáng ngờ trước khi vào đợt
> quyết toán.

SettleGuard chạy phía trên các kênh thanh toán bên ngoài và **không tự chuyển
tiền thật**: hệ thống ghi nhận ai nợ ai bao nhiêu, chấm điểm rủi ro từng giao
dịch, rồi gom các giao dịch "sạch" thành từng đợt quyết toán (settlement).
Doanh nghiệp khách hàng tích hợp qua API; ứng dụng mobile cho đội vận hành
theo dõi trạng thái quyết toán, xem cảnh báo và duyệt/từ chối giao dịch bị
giữ lại.

---

## Mục lục

- [Tổng quan kiến trúc](#tổng-quan-kiến-trúc)
- [Các thành phần](#các-thành-phần)
- [Luồng sự kiện](#luồng-sự-kiện)
- [Quy tắc nghiệp vụ và chấm điểm rủi ro](#quy-tắc-nghiệp-vụ-và-chấm-điểm-rủi-ro)
- [Xác thực](#xác-thực)
- [Chạy trên máy local](#chạy-trên-máy-local)
- [Kiểm thử](#kiểm-thử)
- [Hiệu năng](#hiệu-năng)
- [Cấu trúc thư mục](#cấu-trúc-thư-mục)
- [Giới hạn hiện tại](#giới-hạn-hiện-tại)
- [Tài liệu liên quan](#tài-liệu-liên-quan)

---

## Tổng quan kiến trúc

Kiến trúc **hướng sự kiện (event-driven)**. Mỗi service sở hữu database
Postgres riêng, không service nào đọc thẳng DB của service khác. Các
service trao đổi với nhau qua **NATS JetStream**, không gọi đồng bộ lẫn
nhau. Các service có publish event (ledger, accounts, settlement) đều dùng
**outbox pattern**: ghi dữ liệu và ghi event vào cùng một DB transaction,
sau đó một relay mới đẩy event lên NATS. Nhờ vậy không bao giờ có chuyện ghi
DB xong mà event bị mất.

**Đường đi của request:** gateway là cửa vào dành cho lời gọi từ bên ngoài
(xem lưu ý ở phần [Xác thực](#xác-thực)).

```mermaid
flowchart LR
    client["Client business<br/>(server-to-server)"] --> gw
    mobile["mobile-app<br/>(Expo / TypeScript)"] --> gw
    gw["gateway :8000<br/>API key · CORS · rate limit"]
    gw --> ledger["ledger-service :8080"]
    gw --> accounts["accounts-service :8081"]
    gw --> settle["settlement-engine :8082"]
    gw --> notif["notification-service :8083"]
```

**Luồng event qua NATS JetStream:** một giao dịch được ghi vào sổ cái sẽ
kéo theo việc cập nhật số dư, chấm điểm rủi ro và lưu cảnh báo.

```mermaid
flowchart LR
    ledger["ledger-service"] -- "ledger.entry-recorded" --> accounts["accounts-service<br/>cập nhật số dư"]
    ledger -- "ledger.entry-recorded" --> settle["settlement-engine<br/>chấm điểm rủi ro"]
    settle -- "transaction.risk-scored (hold)<br/>settlement.finalized" --> notif["notification-service<br/>lưu cảnh báo"]
```

---

## Các thành phần

| Thành phần | Ngôn ngữ | Cổng | Vai trò |
|---|---|:---:|---|
| [`services/ledger-service`](services/ledger-service) | Go, Postgres | `8080` | Sổ cái **append-only**, ghi bút toán kép (tổng debit = tổng credit). Nguồn sự thật cho mọi service khác. |
| [`services/accounts-service`](services/accounts-service) | Go, Postgres | `8081` | Danh tính client business và account; duy trì số dư `Σcredit − Σdebit` từ event của ledger. |
| [`services/settlement-engine`](services/settlement-engine) | Go, Postgres | `8082` | Chấm điểm rủi ro real-time cho từng giao dịch, chạy định kỳ để gom giao dịch sạch thành settlement, cho phép duyệt/từ chối giao dịch bị giữ. |
| [`services/notification-service`](services/notification-service) | Python, Postgres | `8083` | Nhận event giữ giao dịch và quyết toán xong, lưu thành bảng notification (audit trail). |
| [`services/gateway`](services/gateway) | Go, Postgres | `8000` | Reverse proxy đứng trước 4 service: xác thực API key, CORS, rate limit, request ID. |
| [`mobile-app`](mobile-app) | Expo / TypeScript | — | Client chủ yếu để xem dữ liệu, kèm thao tác duyệt/từ chối giao dịch đang bị giữ. |

Mỗi service có `README.md` riêng hướng dẫn chạy, test và gọi API. Đặc tả
OpenAPI nằm trong [`docs/openapi/`](docs/openapi).

---

## Luồng sự kiện

| Event | Bên publish | Bên consume | Ý nghĩa |
|---|---|---|---|
| `ledger.entry-recorded` | ledger-service | accounts-service, settlement-engine | Có giao dịch mới được ghi vào sổ cái. |
| `account.updated` | accounts-service | *(chưa có)* | Account thay đổi (số dư, trạng thái). |
| `transaction.risk-scored` | settlement-engine | notification-service | Kết quả chấm điểm: `pass` hoặc `hold`. Chỉ `hold` mới tạo notification. |
| `transaction.resolved` | settlement-engine | *(chưa có)* | Giao dịch bị giữ đã được người duyệt hoặc từ chối. |
| `settlement.finalized` | settlement-engine | notification-service | Một đợt quyết toán đã chốt. |

Mọi consumer đều **idempotent**: JetStream giao event theo kiểu
at-least-once, nên xử lý trùng một event hai lần vẫn cho kết quả như một
lần.

---

## Quy tắc nghiệp vụ và chấm điểm rủi ro

Mọi invariant nghiệp vụ được liệt kê kèm ID trong
[`docs/BUSINESS_RULES.md`](docs/BUSINESS_RULES.md) (ví dụ `LEDGER-01`,
`SETTLEMENT-01`). Test bảo mật và test contract đều tham chiếu tới các ID
này.

Chấm điểm rủi ro hiện tại hoàn toàn **dựa trên rule**. Một giao dịch bị
**giữ lại (hold)** nếu vi phạm **bất kỳ** rule nào:

| Rule | Điều kiện kích hoạt (mặc định) | Trọng số điểm |
|---|---|:---:|
| Velocity limit | Account đã có hơn 5 giao dịch trong 5 phút trước đó | 40 |
| Mismatch threshold | Số tiền vượt 10.000.000 ₫ | 30 |
| Blocklist | Account nằm trong danh sách chặn | 100 |

Điểm rủi ro là tổng trọng số của các rule bị kích hoạt, tối đa 100. Điểm
chỉ dùng để audit; quyết định giữ hay cho qua chỉ phụ thuộc vào việc có rule
nào bị kích hoạt hay không.

Giao dịch bị giữ **không bao giờ** được gom vào settlement cho tới khi có
người duyệt. Duyệt hoặc từ chối chỉ hợp lệ khi giao dịch đang `held`; gọi
lần thứ hai trả về `409`.

> **Đơn vị tiền:** mọi trường số tiền là **số nguyên VND (đồng)**, không
> dùng số thực và không nhân/chia 100 (`MONEY-01`).

---

## Xác thực

| Đối tượng gọi | Cơ chế | Trạng thái |
|---|---|---|
| Client business (server-to-server) | API key, xác thực tại gateway | ✅ Đã có |
| Người dùng mobile / vận hành | JWT, xác thực tại gateway | ⏳ Chưa làm |

- Gateway chỉ lưu **SHA-256 của key**, không lưu key thô. Key được cấp bằng
  CLI `services/gateway/cmd/adminctl` và chỉ hiển thị **một lần** lúc tạo.
- ⚠️ Các cổng `8080`–`8083` của 4 service **không có xác thực**. Chỉ traffic
  đi qua gateway (`:8000`) mới được bảo vệ.
- ⚠️ **Xác thực không phải phân quyền:** một key hợp lệ chưa bị giới hạn
  trong các account của chính client đó. Phân quyền theo quyền sở hữu là
  phần việc chưa làm.

---

## Chạy trên máy local

**Cần có:** Docker, Go 1.26+, Python 3.11+, Node.js (cho `mobile-app`), và
CLI [`migrate`](https://github.com/golang-migrate/migrate) có driver
Postgres (notification-service dùng để chạy migration).

**1. Hạ tầng (Postgres cho từng service + NATS JetStream)**

```bash
# Mỗi service có .env.example chứa thông tin Postgres local; compose đọc file .env
for s in ledger-service accounts-service settlement-engine notification-service gateway; do
  cp services/$s/.env.example services/$s/.env
done

docker compose -f infra/docker/docker-compose.yml up -d
```

| Postgres của | Cổng host |
|---|:---:|
| ledger-service | `5433` |
| accounts-service | `5434` |
| settlement-engine | `5435` |
| notification-service | `5436` |
| gateway | `5437` |

NATS client chạy ở cổng `4222`.

**2. Các service**

Mỗi service đọc `DATABASE_URL` và `NATS_URL` từ biến môi trường. Câu lệnh
chạy đầy đủ cho từng service nằm trong [`tests/README.md`](tests/README.md)
và README của từng service. Ví dụ với ledger-service:

```bash
cd services/ledger-service
export DATABASE_URL="postgres://ledger:ledger@localhost:5433/ledger?sslmode=disable"
export NATS_URL="nats://localhost:4222"
go run ./cmd/server
```

File [`.env.example`](.env.example) ở thư mục gốc liệt kê mọi biến môi
trường mà từng service đọc.

**3. Ứng dụng mobile**

```bash
cd mobile-app
cp .env.example .env   # điền URL của các service
npm install
npm start
```

---

## Kiểm thử

| Bộ test | Công cụ | Số test | Chạy với |
|---|---|:---:|---|
| Go: unit + integration (ledger, accounts, settlement, gateway) | `go test`, testcontainers-go | 192 | Postgres và NATS thật trong container, không mock |
| notification-service | pytest, testcontainers | 22 | Postgres và NATS thật trong container |
| mobile-app | Jest, React Native Testing Library | 33 | API được mock |
| `tests/api`: contract + end-to-end | pytest, httpx | 17 | Cả hệ thống đang chạy |
| `tests/security`: thử vượt rule gian lận | pytest, httpx | 5 | Cả hệ thống đang chạy |
| `tests/perf`: tải cho luồng chấm điểm | Locust | — | Cả hệ thống đang chạy |

```bash
# Go (chạy trong thư mục từng service). -p 1 vì test chạy song song sẽ tranh Docker
go test -count=1 -p 1 ./...

# notification-service
cd services/notification-service && python -m pytest

# mobile-app
cd mobile-app && npm test

# Test xuyên service (cần cả hệ thống đang chạy, xem tests/README.md)
cd tests && pytest api security
```

**CI** ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) chạy trên
mọi pull request và mỗi lần push lên `main`:

- `go vet`, `golangci-lint` và test cho 4 Go module;
- pytest cho notification-service;
- ESLint, Prettier và Jest cho mobile-app;
- dựng toàn bộ hệ thống để chạy `tests/api` và `tests/security`.

---

## Hiệu năng

Baseline đo bằng Locust trên một máy (Ryzen 7 5800H, Windows 11, Docker
Desktop), gửi `POST /transactions` liên tục:

| Users | Requests (60 giây) | req/s | Lỗi | p50 | p95 | p99 |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| 20 | 3.615 | 61,1 | 0% | 10 ms | 35 ms | 52 ms |
| 50 | 8.383 | 142,2 | 0% | 15 ms | 59 ms | 110 ms |
| 100 | 13.154 | 222,2 | 0% | 48 ms | 200 ms | 290 ms |

Phát hiện chính: outbox relay của ledger-service chỉ publish tối đa
**~40 event/giây** (20 event mỗi 500 ms). Khi tải vượt mức này, việc chấm
điểm rủi ro bị trễ dần, nhưng không mất dữ liệu.

Chi tiết, bằng chứng và câu lệnh tái hiện:
[`docs/perf-baseline.md`](docs/perf-baseline.md).

Workflow **Perf smoke** (chạy tay) chạy 20 users trong 30 giây và fail nếu
tỉ lệ lỗi > 1% hoặc p95 > 100 ms.

---

## Cấu trúc thư mục

```text
SettleGuard/
├── services/
│   ├── ledger-service/         # Go — sổ cái append-only
│   ├── accounts-service/       # Go — account và số dư
│   ├── settlement-engine/      # Go — chấm điểm rủi ro, quyết toán
│   ├── notification-service/   # Python — lưu cảnh báo
│   └── gateway/                # Go — API key, CORS, rate limit
├── mobile-app/                 # Expo / TypeScript
├── tests/
│   ├── api/                    # contract + end-to-end
│   ├── security/               # thử vượt rule gian lận
│   └── perf/                   # Locust + kết quả đo
├── infra/docker/               # docker-compose cho môi trường dev
├── docs/                       # quy tắc nghiệp vụ, OpenAPI, thiết kế
└── .github/workflows/          # CI
```

---

## Giới hạn hiện tại

Phạm vi v1 **không** gồm: chuyển tiền thật (giao cho bên xử lý thanh toán
ngoài), đa tiền tệ/FX, triển khai on-premise.

Những phần đã thiết kế nhưng **chưa làm**:

- Chấm điểm rủi ro bằng ML. Hiện chỉ có rule.
- JWT cho người dùng mobile, và phân quyền theo quyền sở hữu account.
- Kênh gửi thông báo thật (email/push). Hiện chỉ lưu vào DB.
- Manifest Kubernetes và Terraform. Hiện chỉ có docker-compose cho dev.
- Test E2E cho mobile (Appium) và công cụ AI sinh test / phân loại lỗi CI
  (`tests/ai-tools`).

---

## Tài liệu liên quan

| Tài liệu | Nội dung |
|---|---|
| [`docs/PROJECT_CHARTER.md`](docs/PROJECT_CHARTER.md) | Mục tiêu và phạm vi dự án |
| [`docs/BUSINESS_RULES.md`](docs/BUSINESS_RULES.md) | Danh sách invariant nghiệp vụ, tra theo ID |
| [`docs/WORDING.md`](docs/WORDING.md) | Giải thích thuật ngữ nghiệp vụ và kỹ thuật |
| [`docs/openapi/`](docs/openapi) | Đặc tả OpenAPI 3.1 của từng service |
| [`docs/perf-baseline.md`](docs/perf-baseline.md) | Kết quả đo hiệu năng |
| [`tests/README.md`](tests/README.md) | Cách dựng hệ thống và chạy test xuyên service |
| [`CLAUDE.md`](CLAUDE.md) | Kiến trúc, stack, quy ước làm việc và git workflow |
