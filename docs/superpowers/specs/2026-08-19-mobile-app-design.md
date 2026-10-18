# mobile-app MVP — Thiết kế

Ngày: 2026-08-19
Trạng thái: Nháp — cần review trước khi chuyển thành implementation plan

## 1. Bối cảnh & Phạm vi

`mobile-app` là mảnh cuối cùng còn là scaffold trống trong kiến trúc 5
service của SettleGuard. Theo charter
(`docs/superpowers/specs/2026-07-29-project-charter-design.md`, mục "Ứng
dụng di động"): app giao tiếp với accounts/ledger/settlement-engine qua
một API hướng đọc, nhận push notification từ notification-service, và
**không bao giờ là nguồn sự thật cho dữ liệu miền**. Tiêu chí thành công
v1 của charter còn nêu rõ: *"Các giao dịch bị giữ lại/gắn cờ được hiển thị
và có thể thao tác được (approve/reject) từ ứng dụng di động"*.

### Khoảng trống với backend hiện tại (quan trọng — chặn phần lớn plan)

Kiểm tra lại 4 service đã có MVP cho thấy **không đủ HTTP surface** để
mobile-app hoàn thành phạm vi charter mô tả:

| Service | Có sẵn | Thiếu |
|---|---|---|
| accounts-service | `GET /accounts/{id}`, `GET /accounts?client_id=`, `GET /clients/{id}` | — (đủ dùng) |
| ledger-service | `GET /entries?account_id=` hoặc `?transaction_id=` | — (đủ dùng) |
| settlement-engine | chỉ `GET /health` (plan MVP loại trừ tường minh "any HTTP surface beyond GET /health") | list/detail transaction, list/detail settlement, và quan trọng nhất: **API approve/reject cho transaction đang `held`** — trạng thái `held` hiện được comment thẳng trong code là *"terminal-for-now state (no resolution path yet in this MVP)"* (`services/settlement-engine/internal/settlement/transaction_repository.go`) |
| notification-service | chỉ `GET /health` (spec MVP loại trừ tường minh "Endpoint HTTP để ops team query lịch sử notification... chưa có use case cụ thể yêu cầu ở v1") | list notification/alert đã ghi trong bảng `notifications` |

Nói cách khác: hai trong ba nguồn dữ liệu mobile-app cần (settlement status
+ risk alerts, và toàn bộ hành động approve/reject) **chưa tồn tại ở phía
server**. Đây không phải lỗi của các plan trước — cả hai đều loại trừ HTTP
surface này *tường minh, có chủ đích*, đúng tinh thần MVP-rồi-defer của dự
án, với lý do "chưa có consumer nào cần" — mobile-app chính là consumer đó,
giờ mới xuất hiện.

### Quyết định phạm vi

Theo Git Workflow của repo ("mỗi commit phải nằm đúng branch theo phạm vi
của nó — không commit cross-scope"), việc thêm endpoint cho
settlement-engine/notification-service **không thuộc về** plan/branch của
mobile-app. Vì vậy:

1. Spec này định nghĩa **hợp đồng API tối thiểu** mà settlement-engine và
   notification-service cần bổ sung (liệt kê ở mục 4) — đóng vai trò đặc
   tả cho hai plan riêng, nhỏ, sẽ viết sau, mỗi cái trên branch
   `service/settlement-engine` và `service/notification-service` hiện có
   (mở rộng MVP đã merge, không phải service mới).
2. Plan implementation đi kèm spec này
   (`docs/superpowers/plans/2026-08-19-mobile-app-mvp.md`) **chỉ** động vào
   `mobile-app/`, và giả định hai API bổ sung đó đã tồn tại — nên hai plan
   backend nhỏ ở trên phải chạy xong (hoặc ít nhất merge) trước khi task
   nào trong plan mobile-app gọi tới `POST /transactions/{id}/approve`
   hay `GET /notifications` được thực thi thật.

Coding mode: Claude code trực tiếp (theo working-style hiện tại — `Claude
codes autonomously` cho `mobile-app`).

## 2. Kiến trúc & Tech Stack

Không có tiền lệ React Native nào trong repo — các lựa chọn dưới đây dịch
tinh thần "đơn giản, không framework nặng tùy tiện" mà 4 service kia đã
theo, sang hệ sinh thái RN:

- **Expo (managed workflow) + TypeScript** — không cần native module nào
  cho v1 (app chỉ fetch API + render list), nên không có lý do bắt đầu từ
  bare React Native CLI (setup Xcode/Android Studio phức tạp hơn, không
  mang lại lợi ích nào ở bước này). TypeScript để nhất quán tinh thần
  "type hint ở mọi nơi liên quan tới tiền/risk" đã quy định trong
  `docs/CODING_STANDARDS.md` cho Go/Python — TS là điều tương đương tự
  nhiên cho RN.
- **React Navigation** (`@react-navigation/native` + `native-stack`) — thư
  viện điều hướng chuẩn của hệ sinh thái RN, không tự viết router riêng.
- **TanStack Query** (`@tanstack/react-query`) cho data-fetching — v1 cần
  polling định kỳ cho màn Alerts (xem mục 4, chưa có push thật) và cache
  giữa các màn hình dùng chung dữ liệu account; đây là nhu cầu cụ thể, có
  thật, không phải abstraction thêm cho vui theo nguyên tắc "simplicity
  over cleverness — tránh abstraction tới khi có nhu cầu cụ thể".
  **Không** dùng Redux/MobX — không có state phức tạp nào cần một state
  management layer riêng ở v1 (toàn bộ state là server state do React
  Query quản).
- Gọi thẳng `fetch` built-in (không thêm axios) — không có nhu cầu nào
  (interceptor, cancel token phức tạp) mà `fetch` + React Query không tự
  lo được.
- **Không có gateway.** Charter để ngỏ lựa chọn "đứng sau gateway hoặc do
  settlement-engine phục vụ trực tiếp — quyết định thuộc implementation
  plan". Với đúng 4 base URL cấu hình qua env, một mobile client duy nhất,
  không cần gateway ở v1 — thêm gateway bây giờ là abstraction chưa có nhu
  cầu cụ thể. App gọi trực tiếp 4 service qua 4 base URL riêng.
- **Không auth** ở v1 — nhất quán với việc auth đã bị hoãn ở cả 4 service
  backend ("giống các service khác, hoãn tới khi có quyết định chung về
  auth" — xem spec accounts-service). App không có màn login; người dùng
  tự nhập Client ID để xem dữ liệu (xem mục 3).
- **Không push notification thật.** notification-service tự nó đã hoãn
  delivery thật vì "không có contact info nào trong hệ thống... Push cần
  mobile-app (chưa tồn tại)". Giờ mobile-app tồn tại nhưng vẫn chưa có: (a)
  provider push nào được chọn (Expo push service là ứng viên tự nhiên
  nhưng chưa quyết), (b) cơ chế đăng ký/lưu device token ở
  notification-service. Quyết định thật thay vì giả vờ có push: **v1 dùng
  polling** (React Query, `refetchInterval` ~15s khi màn Alerts đang mở)
  gọi `GET /notifications` — đủ để thỏa "thấy fraud/risk alert theo thời
  gian thực" ở mức chấp nhận được cho MVP, không cần dựng hạ tầng push
  chưa ai yêu cầu cụ thể.
- Layout: `mobile-app/App.tsx` (entrypoint) +
  `mobile-app/src/{api,screens,components,navigation,config}/` — mirror
  tinh thần `cmd/server/main.go` + `internal/{...}/` bên Go, dịch sang cấu
  trúc RN thông dụng.

## 3. Domain Model & Screens

mobile-app không sở hữu domain entity nào (đúng vai trò "never a source of
truth"). Nó render lại 5 loại dữ liệu đọc từ backend:

| Màn hình | Nguồn dữ liệu | Hành động |
|---|---|---|
| **Client Lookup** (màn khởi đầu) | Nhập tay Client ID (không có login) | Điều hướng sang Account List |
| **Account List** | `GET /accounts?client_id=` (accounts-service) | Chọn 1 account → Account Detail |
| **Account Detail** | `GET /accounts/{id}` (accounts-service) + `GET /entries?account_id=` (ledger-service) | Xem balance + lịch sử ledger entry |
| **Held Transactions** | `GET /transactions?status=held` (settlement-engine, **mới**) | Approve / Reject từng transaction (`POST /transactions/{id}/approve\|reject`, **mới**) |
| **Settlements** | `GET /settlements` (settlement-engine, **mới**) | Xem list batch đã finalize (read-only) |
| **Alerts** | `GET /notifications` (notification-service, **mới**), polling | Read-only, đánh dấu đã xem chỉ ở local state (không có PATCH nào ở backend) |

Held Transactions là màn hình quan trọng nhất về mặt nghiệp vụ — đây là nơi
duy nhất trong toàn hệ thống có hành động **ghi** (không chỉ đọc) từ phía
mobile-app, khớp đúng tiêu chí thành công charter đã nêu.

## 4. Hợp đồng API cần bổ sung ở backend (đặc tả cho 2 plan riêng)

### settlement-engine (mở rộng MVP hiện có, branch `service/settlement-engine`)

- `GET /transactions?status=held` → `200` list `Transaction` (id, amount,
  score, decision, status, triggered_rules, scored_at, account_ids — join
  `transaction_accounts`).
- `GET /transactions/{id}` → `200` Transaction hoặc `404`.
- `POST /transactions/{id}/approve` → chuyển `held` → `pending_settlement`
  (để vào batch settle tiếp theo bình thường, không có luồng payout riêng).
  `409` nếu status hiện tại không phải `held`. `404` nếu không tồn tại.
- `POST /transactions/{id}/reject` → chuyển `held` → trạng thái mới
  `rejected` (cần thêm vào CHECK constraint của cột `status`, migration
  mới) — **terminal thật sự**, không bao giờ vào batch. `409`/`404` tương
  tự approve.
- `GET /settlements` → `200` list Settlement (id, transaction_count,
  total_amount, created_at).
- `GET /settlements/{id}` → `200` Settlement + list transaction_id hoặc
  `404`.

Đây là thay đổi nghiệp vụ thật (`held` từ "terminal-for-now" thành có
resolution path) — cần thêm invariant mới vào `docs/BUSINESS_RULES.md`
(vd `SETTLEMENT-0x` — approve/reject chỉ hợp lệ từ status `held`) khi
implement, và cân nhắc: approve/reject có publish event không (khả năng
cao — `notification-service` hoặc future service khác có thể muốn biết
kết quả resolution). Để hai plan riêng đó quyết định chi tiết.

### notification-service (mở rộng MVP hiện có, branch `service/notification-service`)

- `GET /notifications?type=&since=` → `200` list Notification (id, type,
  subject_id, payload, created_at), sort theo `created_at DESC`, có phân
  trang tối thiểu (`limit`, mặc định vd 50) vì bảng này không có TTL/dọn
  dẹp. Cần thêm `http.server` router thật hoặc handler thủ công (hiện tại
  `internal/api/health.py` chỉ có 1 route `/health` — mở route thứ hai vẫn
  hợp lý với stdlib, chưa cần FastAPI/Flask).

## 5. Testing

- **Unit/component** (Jest + `@testing-library/react-native`) — test các
  hook data-fetching thuần (map response → view model) và các component
  hiển thị (vd `HeldTransactionCard` render đúng amount/score, gọi đúng
  callback khi bấm Approve/Reject) mà không cần network thật — mock ở
  boundary `fetch`, không mock sâu hơn (nhất quán tinh thần "no mocks" của
  repo, áp dụng ở mức boundary ngoài cùng vì RN component test không có
  lựa chọn nào chạm network thật một cách hợp lý).
- **E2E (Appium, Python)** — theo `tests/` layout đã định
  (`docs/superpowers/specs/2026-07-29-project-charter-design.md`, mục
  Testing cho mobile): điều khiển bản build thật của app. **Nằm ngoài
  phạm vi plan mobile-app MVP này** — cần cả app lẫn backend thật chạy
  đồng thời (docker-compose full stack), và thư mục `tests/` nói chung
  chưa tồn tại trong repo (không riêng gì phần mobile). Để lại thành một
  bước sau, sau khi `tests/` được scaffold cho `tests/api` trước (mảnh có
  giá trị sớm nhất vì đã có OpenAPI-able surface ở cả 4 service).

## 6. Tiêu chí thành công cho MVP này

- App chạy được trên Expo Go (hoặc simulator/emulator qua `expo start`).
- Nhập Client ID hợp lệ → thấy đúng danh sách Account của client đó.
- Chọn 1 Account → thấy đúng balance hiện tại + lịch sử ledger entry.
- Màn Held Transactions hiển thị đúng các transaction đang `held`; bấm
  Approve/Reject gọi đúng API mới, list tự cập nhật (invalidate React
  Query cache) sau khi thao tác thành công.
- Màn Settlements hiển thị đúng list batch đã finalize.
- Màn Alerts hiển thị notification mới nhất, tự làm mới theo polling
  interval mà không cần người dùng kéo refresh thủ công.
- Test Jest pass (`npm test`).

## 7. Để lại cho việc khác (không nằm trong spec này)

- Hai plan backend ở mục 4 (chặn các task liên quan Held
  Transactions/Settlements/Alerts của plan mobile-app — các task Account
  List/Detail không bị chặn, có thể làm trước).
- Push notification thật (Expo push token registration + provider ở
  notification-service) — cần quyết định provider trước.
- Auth/đăng nhập thật cho mobile-app (đang dùng nhập tay Client ID làm
  placeholder không có xác thực).
- Appium E2E suite (`tests/`).
- Build production (EAS Build), publish lên App Store/Play Store, k8s
  manifest liên quan (mobile-app không cần k8s vì không phải service phía
  server).
