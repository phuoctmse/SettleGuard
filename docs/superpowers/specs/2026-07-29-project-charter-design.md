# SettleGuard Project Charter — Thiết kế

Ngày: 2026-07-29
Trạng thái: Đã duyệt (đang chờ review spec cuối cùng)

## 1. Tầm nhìn & Phạm vi

**SettleGuard** là một nền tảng B2B theo dõi và tất toán (settle) các nghĩa vụ
tài chính giữa các bên, dựa trên hạ tầng thanh toán bên ngoài (external
payment rails) — bản thân nó không bao giờ di chuyển tiền thật — đồng thời
chủ động giám sát quá trình tất toán bằng chấm điểm rủi ro (risk scoring) dựa
trên rule và ML. Các doanh nghiệp khách hàng tích hợp qua API để tất toán
payment/payout giữa người dùng của chính họ. Ứng dụng di động riêng của
SettleGuard cho phép end-user (hoặc đội vận hành của khách hàng) theo dõi
trạng thái tất toán và xem cảnh báo gian lận/rủi ro theo thời gian thực.

### Trong phạm vi (v1)

- Theo dõi và tất toán nghĩa vụ đơn tiền tệ (single-currency) (accounts,
  ledger, settlement-engine)
- Chấm điểm rủi ro theo rule + ML theo thời gian thực khi giao dịch phát sinh
- Các đợt tất toán theo lô (batch), hoàn tất các nghĩa vụ đã được xác định
  là không rủi ro (risk-cleared)
- Thông báo (push/email) cho các sự kiện tất toán và các trường hợp bị giữ
  lại do rủi ro (risk hold)
- Ứng dụng di động để xem trạng thái tất toán/cảnh báo
- Triển khai trên cloud (Kubernetes + Terraform)

### Ngoài phạm vi (v1) — các mục tiêu không hướng tới, nêu rõ

- Di chuyển tiền thật trực tiếp (không giữ tiền/custody, không thực thi
  payout — việc này giao cho một payment processor bên ngoài)
- Đa tiền tệ / FX
- Triển khai on-premise

## 2. Kiến trúc & Mô hình miền (Domain Model)

### Các service (lõi hướng sự kiện — event-driven)

- **accounts-service** (Go) — sở hữu định danh party/account, số dư nghĩa vụ
  (balances-of-obligation, không phải tiền thật), và trạng thái account.
  Phát sinh (publish) sự kiện `account.updated`.
- **ledger-service** (Go) — nguồn sự thật duy nhất, chỉ-ghi-thêm
  (append-only) cho mọi bút toán nghĩa vụ (kiểu double-entry: ai nợ ai, vì
  sao, trạng thái hiện tại). Phát sinh sự kiện `ledger.entry-recorded`. Dùng
  Postgres.
- **settlement-engine** (Go) — bộ điều phối trung tâm (core orchestrator).
  Tiêu thụ (consume) sự kiện từ ledger, chạy chấm điểm rủi ro theo rule + ML
  theo thời gian thực cho từng giao dịch, phát sinh sự kiện
  `transaction.risk-scored` (mang theo điểm số + quyết định giữ lại/thông
  qua). Theo lịch định kỳ, gom tất cả các bút toán đã được thông qua kể từ
  lần chạy trước thành một đợt tất toán (settlement), phát sinh sự kiện
  `settlement.finalized` hoặc `settlement.held-for-review`.
- **notification-service** (Python) — subscribe vào các sự kiện risk-hold và
  settlement-finalized; gửi cảnh báo push/email tới ứng dụng di động và/hoặc
  webhook của khách hàng. Không bao giờ được gọi đồng bộ (synchronously) bởi
  service khác.

### Quyền sở hữu dữ liệu (Data ownership)

Mỗi service sở hữu schema riêng của nó; không service nào được truy cập
trực tiếp database của service khác. Postgres được dùng cho các service cần
lưu trữ dữ liệu (tối thiểu là ledger-service và accounts-service).

### Event bus

Một event broker (công nghệ cụ thể, ví dụ Kafka hoặc dịch vụ quản lý tương
đương, là quyết định thuộc về implementation plan, không phải quyết định ở
tầng charter) là trục xương sống kết nối bốn service. Đây chính là yếu tố
giúp mô hình lai giữa chấm điểm thời gian thực và tất toán theo lô hoạt động
tự nhiên: việc chấm điểm phản ứng liên tục với luồng giao dịch, còn tất toán
định kỳ "rút" ra một cửa sổ (window) các bút toán đã được thông qua từ chính
luồng đó.

### Các thực thể miền (domain entities) chính

Account, LedgerEntry, Transaction, RiskScore, Settlement (batch),
Alert/Notification.

### Ứng dụng di động (React Native)

Giao tiếp với accounts/ledger/settlement-engine qua một API hướng đọc
(read-oriented) (có thể đứng sau một gateway hoặc do settlement-engine phục
vụ trực tiếp — đây là quyết định thuộc về implementation plan) và nhận push
notification từ notification-service. Ứng dụng di động không bao giờ là
nguồn sự thật (source of truth) cho dữ liệu miền.

## 3. Testing, Tooling & Tiêu chí thành công

### Cấu trúc test (khớp với khung `tests/` hiện có)

- `tests/api` — test API/contract liên service (Python), chạy dựa trên
  OpenAPI spec (sẽ được viết cho từng service như một bước tiếp theo sau
  charter này)
- `tests/security` — test tập trung vào bảo mật (thử bypass gian lận, kiểm
  tra ranh giới xác thực/phân quyền)
- `tests/perf` — test tải/hiệu năng cho đường chấm điểm thời gian thực của
  settlement-engine và thông lượng tất toán theo lô
- `tests/ai-tools` — tooling Python hỗ trợ AI để sinh test case và phân
  loại (triage) lỗi CI (khác với chính mô hình ML chấm điểm rủi ro, vốn nằm
  bên trong settlement-engine)

### Testing cho mobile

Appium (Python client) điều khiển ứng dụng React Native để chạy test
end-to-end.

### Tiêu chí thành công cho v1

- Một giao dịch chạy trọn vẹn từ đầu đến cuối: được ghi vào ledger → được
  chấm điểm rủi ro theo thời gian thực → được đưa vào đợt tất toán theo lô
  tiếp theo (hoặc bị giữ lại) → gửi thông báo
- Các giao dịch bị giữ lại/gắn cờ được hiển thị và có thể thao tác được
  (approve/reject) từ ứng dụng di động
- Cả bốn service đều có thể triển khai độc lập qua Kubernetes manifest được
  sinh ra từ hạ tầng do Terraform cấp phát

## 4. Tóm tắt Tech Stack

| Thành phần            | Stack                                    |
|-----------------------|-------------------------------------------|
| accounts-service      | Go, Postgres                              |
| ledger-service        | Go, Postgres                              |
| settlement-engine     | Go, Postgres (hoặc event-store), mô hình chấm điểm ML |
| notification-service  | Python                                    |
| mobile-app            | React Native                              |
| Test automation       | Python (API, security, perf, AI tooling); Appium cho mobile E2E |
| Infra                 | Kubernetes (`infra/k8s`), Terraform (`infra/terraform`) |

## 5. Để lại cho Implementation Plan

Những mục sau đây được cố ý để ngỏ ở đây và sẽ được quyết định khi từng
sub-project (theo từng service) được lập plan:

- Lựa chọn event broker cụ thể và thiết kế topic/schema
- Contract OpenAPI cho từng service (`docs/openapi.yaml` hiện tại chỉ là
  placeholder rỗng; cần được tách hoặc điền nội dung theo từng service)
- Lựa chọn mô hình chấm điểm rủi ro ML, dữ liệu huấn luyện, và cách serving
- Thiết kế gateway/API-facade cho ứng dụng di động
- Chi tiết pipeline CI/CD ngoài phần khung k8s/terraform đã có sẵn
