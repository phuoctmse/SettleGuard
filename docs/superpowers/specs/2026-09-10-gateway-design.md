# services/gateway — Thiết kế

Ngày: 2026-09-10
Trạng thái: Nháp — cần review trước khi chuyển thành implementation plan

## 1. Bối cảnh & Phạm vi

Hiện **không service nào có xác thực**. `POST /transactions` của
`ledger-service` — endpoint ghi vào sổ cái append-only — đang mở cho bất kỳ
ai gọi được tới cổng. Đây là khoảng cách lớn nhất giữa hệ thống hiện tại và
bất cứ thứ gì deploy được (xem `CLAUDE.md` § Architecture, quyết định auth
ngày 2026-09-09).

Song song, `mobile-app` đang gọi thẳng 4 base URL riêng
(`EXPO_PUBLIC_{ACCOUNTS,LEDGER,SETTLEMENT,NOTIFICATION}_API_URL`). React
Native không bị CORS nên chưa lộ vấn đề, nhưng **không service nào có CORS
middleware** — một frontend web sẽ bị trình duyệt chặn ngay ở request đầu
tiên.

`services/gateway` giải quyết cả hai: một reverse proxy mỏng đặt trước 4
service, làm một chỗ duy nhất cho CORS, xác thực API key, rate limit và
request ID.

**Trong phạm vi:** reverse proxy có tiền tố theo service, CORS, API key cho
client business, rate limit 2 tầng, request ID, một CLI quản trị để cấp và
thu hồi key.

**Ngoài phạm vi, có lý do:**

- **JWT và danh tính người dùng.** Hệ thống hiện **không có khái niệm người
  dùng** ở bất kỳ đâu — không bảng users, không login, không role.
  `ClientBusiness` là một *doanh nghiệp*, không phải một con người. JWT cho
  mobile/ops kéo theo cả một subsystem danh tính (lưu mật khẩu, endpoint
  đăng nhập, refresh token, phân quyền) xứng đáng có spec riêng thay vì bị
  nhét kèm.
- **Phân quyền theo quyền sở hữu account.** Xem §10.1 — vấn đề thật, đã ghi
  nhận, cần spec riêng.
- **Chặn network để 4 service không còn expose ra ngoài.** Thuộc phần devops
  người dùng tự quản (`CLAUDE.md` § Working Style).

## 2. Bốn quyết định nền và lý do

### 2.1 Gateway tự sở hữu bảng key

Gateway có Postgres riêng với bảng `api_keys`. Xác thực là một truy vấn cục
bộ — không gọi service khác, không phụ thuộc event.

Phương án "accounts-service giữ key, gateway gọi đồng bộ" bị loại vì vi phạm
nguyên tắc event-driven của `CLAUDE.md`, thêm độ trễ vào **mọi** request, và
biến accounts-service thành điểm chết của toàn bộ API.

Phương án "accounts-service giữ key + publish event, gateway cache" bị loại
vì `client_repository.go` hiện **không publish event nào cả** (chỉ
`account_repository.go` có, 3 chỗ), nên phải thêm hẳn một họ event
`client.*` mới; cộng thêm việc thu hồi key chỉ có hiệu lực sau vài giây.

Điều làm phương án này hợp lệ về mặt kiến trúc: **gateway là edge component,
không phải domain service.** Nó không sở hữu thực thể nghiệp vụ nào, không
consume NATS, không publish event. Nguyên tắc "không gọi đồng bộ giữa các
service" nói về các *domain service*; gateway không có domain.

Kèm theo đó là một phân tách khái niệm cần nói rõ: **`ClientBusiness.status
= suspended` là trạng thái nghiệp vụ; `api_keys.revoked_at` là trạng thái
truy cập.** Hai thứ khác nhau, hai bảng khác nhau, cả hai đều tường minh.
Suspend một client **không** tự động chặn key của họ — muốn chặn thì thu hồi
key. Nếu về sau cần hai thứ này liên động, đó là lúc thêm event
`client.updated`.

### 2.2 Định tuyến bằng tiền tố theo service

```
/ledger/*        -> ledger-service        :8080
/accounts/*      -> accounts-service      :8081
/settlement/*    -> settlement-engine     :8082
/notifications/* -> notification-service  :8083
```

Bắt buộc phải có tiền tố vì `ledger-service` phục vụ `POST /transactions`
còn `settlement-engine` phục vụ `GET /transactions` — **trùng path, hai
service, ngữ nghĩa không liên quan**. Định tuyến theo method là luật ngầm dễ
gây nhầm và sẽ vỡ ngay khi một trong hai service thêm method mới trên path
đó.

Bốn service giữ nguyên API của chúng, nên 4 spec OpenAPI ở `docs/openapi/`
vẫn đúng với tư cách hợp đồng *của từng service*. `mobile-app` bỏ 4 biến môi
trường, còn 1 base URL.

### 2.3 Cấp key bằng CLI quản trị, không có endpoint HTTP

`cmd/adminctl` với 3 lệnh: tạo key cho một `client_id`, thu hồi key, liệt kê
key. Ghi thẳng vào DB của gateway.

Thoát hẳn nghịch lý khởi tạo: một endpoint cấp key nếu để mở thì ai cũng tự
cấp key cho mình, còn bảo vệ nó thì cần một danh tính admin — thứ đã hoãn ở
§1. Phương án "endpoint admin bảo vệ bằng admin key tĩnh" bị loại vì nó đưa
vào đúng cái phản mẫu vừa loại bỏ (secret tĩnh dùng chung) và không truy
được ai đã cấp key nào.

### 2.4 Cổng 8000

Không đụng dải `:8080-:8083` của 4 service.

## 3. Kiến trúc

Go module thứ tư, cùng khuôn ba service kia:

```
services/gateway/
  cmd/server/main.go
  cmd/adminctl/main.go
  internal/api/       router, middleware
  internal/auth/      sinh key, băm, tra cứu
  internal/proxy/     cấu hình ReverseProxy
  internal/db/        kết nối + migrations/
  internal/testutil/  testcontainers Postgres
```

Module path `github.com/phuoctmse/settleguard/gateway`, `chi` router,
`golang-migrate`, `testcontainers-go` — giống hệt quy ước trong `CLAUDE.md`
§ Stack.

Proxy dùng `net/http/httputil.ReverseProxy` của thư viện chuẩn. Không dùng
framework gateway ngoài: nhu cầu ở đây là 4 tầng middleware và một bảng định
tuyến tĩnh, `ReverseProxy` thừa sức, và `CLAUDE.md` yêu cầu ưu tiên đơn
giản.

Gateway kế thừa graceful shutdown và HTTP timeout đã áp cho 3 service kia
(PR #18) — cùng bộ hằng số, cùng shape.

## 4. Chuỗi middleware

```
RequestID -> CORS -> RateLimit(IP) -> APIKeyAuth -> RateLimit(client) -> Proxy
```

Thứ tự có chủ đích:

- **RequestID đứng đầu** để cả log của 401 và 429 cũng có ID truy vết. Sinh
  UUID nếu client chưa gửi `X-Request-Id`; chuyển tiếp lên upstream và trả
  lại trong response.
- **CORS trước auth.** Preflight `OPTIONS` của trình duyệt **không mang API
  key**. Đặt CORS sau auth là chặn nhầm mọi request từ trình duyệt. Origin
  lấy từ biến môi trường dạng allowlist; **không dùng `*`** vì request có
  mang credential.
- **RateLimit theo IP trước auth** để kẻ chưa có key vẫn bị giới hạn khi dò
  key. Chỉ giới hạn theo client là bỏ ngỏ hoàn toàn việc dò.
- **RateLimit theo client sau auth**, khi đã biết `client_id`.
- **APIKeyAuth** đọc `Authorization: Bearer <key>`, tra `api_keys`, gắn
  `client_id` vào request context.

`GET /health` của chính gateway là endpoint duy nhất không cần key.

### 4.1 Cấu hình

Đọc từ biến môi trường, cùng khuôn `DATABASE_URL` / `LISTEN_ADDR` của 3
service kia. Giá trị mặc định là điểm khởi đầu hợp lý, chỉnh được khi vận
hành thấy cần:

| Biến | Mặc định | Ý nghĩa |
|------|----------|---------|
| `DATABASE_URL` | (bắt buộc) | Postgres của gateway |
| `LISTEN_ADDR` | `:8000` | Cổng gateway |
| `LEDGER_UPSTREAM` | `http://localhost:8080` | Đích của `/ledger/*` |
| `ACCOUNTS_UPSTREAM` | `http://localhost:8081` | Đích của `/accounts/*` |
| `SETTLEMENT_UPSTREAM` | `http://localhost:8082` | Đích của `/settlement/*` |
| `NOTIFICATIONS_UPSTREAM` | `http://localhost:8083` | Đích của `/notifications/*` |
| `CORS_ALLOWED_ORIGINS` | (rỗng = chặn hết) | Danh sách origin, phân tách bằng dấu phẩy |
| `RATE_LIMIT_IP_PER_MINUTE` | `60` | Ngưỡng tầng 1, theo IP, trước auth |
| `RATE_LIMIT_CLIENT_PER_MINUTE` | `600` | Ngưỡng tầng 2, theo client, sau auth |
| `UPSTREAM_TIMEOUT_SECONDS` | `30` | Quá hạn thì trả 504 |

`CORS_ALLOWED_ORIGINS` mặc định **rỗng nghĩa là chặn mọi origin**, không
phải cho qua hết — cấu hình thiếu phải fail-closed, không được im lặng mở
toang.

Ngưỡng IP thấp hơn ngưỡng client 10 lần là có chủ đích: một client hợp lệ
thường gọi từ vài IP server cố định với lưu lượng cao, còn một IP lạ chưa
auth thì không có lý do gì gọi nhiều — đó chính là hình dạng của việc dò
key.

## 5. X-Client-Id

Sau khi xác thực, gateway đặt `X-Client-Id` = client_id của key, rồi chuyển
tiếp lên upstream.

**Gateway phải ghi đè header này vô điều kiện, kể cả khi client tự gửi lên.**
Đây là bẫy bảo mật thật: nếu cài theo kiểu "chỉ thêm nếu chưa có", kẻ tấn
công tự đặt `X-Client-Id` là mạo danh được ngay khi service bắt đầu tin
header đó. Phải luôn `Set`, không bao giờ `Add`, và phải có test riêng cho
trường hợp client gửi header giả.

Ở spec này **chưa service nào tin header đó** — nó tồn tại để hợp đồng có
sẵn, log truy được request về client, và việc enforce ở §10.1 sau này không
phải sửa gateway.

## 6. Định dạng key và cách băm

Key: `sg_live_` + 43 ký tự base64url = 256 bit ngẫu nhiên từ `crypto/rand`.
Tiền tố giúp nhận diện khi key lọt vào log hay repo.

Băm bằng **SHA-256, không phải bcrypt/argon2**. Điểm này phản trực giác nên
phải ghi rõ lý do:

1. bcrypt tồn tại để chống brute-force **mật khẩu do người đặt** (entropy
   thấp). Key ở đây là 256 bit ngẫu nhiên — brute-force bất khả thi, nên chi
   phí làm chậm của bcrypt không mua được gì.
2. Quan trọng hơn: **bcrypt có salt riêng mỗi hàng nên không tra cứu được
   bằng index.** Muốn xác thực phải quét toàn bộ bảng và so từng hàng.
   SHA-256 cho phép `UNIQUE INDEX` trên cột hash, tra cứu O(1).

So sánh hash dùng `crypto/subtle.ConstantTimeCompare`.

Key **chỉ hiện một lần duy nhất** lúc `adminctl` tạo ra; sau đó DB chỉ còn
hash, không có đường khôi phục.

## 7. Schema

```sql
CREATE TABLE api_keys (
    id          UUID PRIMARY KEY,
    client_id   UUID NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_client_id ON api_keys(client_id);
```

`client_id` cố ý **không** là foreign key: accounts-service sở hữu bảng
`client_businesses` ở database khác, và `CLAUDE.md` cấm truy cập DB chéo
service. Gateway chỉ mang theo định danh, không nhân bản danh tính client.

`revoked_at IS NULL` nghĩa là còn hiệu lực. Không xoá hàng — giữ lại để
audit.

`label` (ví dụ `"mobile-app prod"`) để người vận hành phân biệt nhiều key của
cùng một client mà không cần lộ key.

## 8. Xử lý lỗi

Body lỗi dùng đúng format `{"error": "..."}` của 3 service Go, để client chỉ
cần một bộ parser.

| Mã | Khi nào |
|----|---------|
| 401 | Thiếu key, key sai, hoặc key đã thu hồi |
| 429 | Vượt rate limit (IP hoặc client) |
| 404 | Path không khớp tiền tố service nào |
| 502 | Upstream không kết nối được |
| 504 | Upstream timeout |

**401 dùng chung một thông điệp cho cả ba trường hợp** — không tiết lộ key có
tồn tại hay không. 502/504 không để lộ lỗi nội bộ của service ra ngoài.

## 9. Kiểm thử

`testcontainers-go` với Postgres thật, không mock — giống 3 service kia.
Upstream giả bằng `httptest.Server` để test proxy mà không cần dựng cả 4
service.

Ca bắt buộc:

- Key hợp lệ đi qua tới upstream đúng service theo tiền tố
- Key sai / thiếu / đã thu hồi: đều 401, **thông điệp giống hệt nhau**
- Preflight `OPTIONS` qua được khi **không** có key
- Origin ngoài allowlist bị từ chối
- `X-Request-Id` được sinh khi thiếu, và được giữ nguyên khi client gửi
- **Client gửi `X-Client-Id` giả bị ghi đè** (ca chống mạo danh ở §5)
- Rate limit IP chặn đúng ngưỡng khi chưa auth
- Rate limit client chặn đúng ngưỡng sau khi auth
- Proxy giữ nguyên method, body, query string và status code của upstream
- `adminctl`: tạo key trả về key thô đúng một lần; thu hồi rồi thì 401

## 10. Giới hạn đã biết

### 10.1 Xác thực không phải phân quyền

Sau spec này ta biết **request đến từ client nào**, nhưng **không** ngăn được
client A thao tác trên account của client B.

Lỗ hổng cụ thể: A gọi `POST /ledger/transactions` với `account_id` của B.
`ledger-service` không có khái niệm client, nó chỉ INSERT. Sau đó
`accounts-service` consume `ledger.entry-recorded` và cập nhật số dư **của
B**. A vừa dịch chuyển nghĩa vụ tài chính trên tài khoản của B.

Chỉ gắn nhãn `client_id` lên bút toán lúc ghi là **không đủ** — phải chặn tại
thời điểm ghi, mà muốn chặn thì phải biết account nào thuộc client nào.

Chỗ đặt phép kiểm tra **không phải gateway**: gateway sẽ phải đọc và hiểu
body của từng service (`POST /transactions` là JSON lồng, `GET /entries` là
query param, `GET /settlements/{id}` không gắn với account nào), dần biến
thành nơi chứa tri thức nghiệp vụ của cả 4 service. Sai tầng.

Chỗ đúng là từng service tự enforce, và có một chi tiết làm việc này rẻ hơn
thoạt nhìn: **`account.updated` đã mang sẵn cả `AccountID` lẫn `ClientID`**
(`services/accounts-service/internal/account/outbox.go`, payload là snapshot
đầy đủ). Không cần thêm event mới. `ledger-service` và `settlement-engine`
mỗi bên consume `account.updated`, dựng bảng chiếu
`account_owners(account_id, client_id)` cục bộ, rồi từ chối request tham
chiếu account không thuộc client đang gọi — đúng khuôn CROSS-01/CROSS-02.

**`notification-service` cũng thuộc phạm vi này, và cần nhiều hơn hai
service kia** (phát hiện qua security review 2026-09-12, bổ sung sau khi
spec được viết). Bảng `notifications` không có cột tenant nào và
`repository.list` là `WHERE TRUE` chỉ lọc theo `type`/`since`/`limit`, nên
`GET /notifications` trả về cho **bất kỳ ai gọi được** toàn bộ
`account_ids`, `amount`, `score`, `decision` và `triggered_rules` của mọi
tenant. Trường cuối nặng nhất: nó tiết lộ chính xác rule chống gian lận nào
đã bắt giao dịch nào ở mức tiền nào — đủ để dò ngược
`SETTLEMENT_MISMATCH_THRESHOLD` và cửa sổ velocity mà không cần đoán mò.

Auth ở gateway **không** sửa được lỗi này, vì không có cột nào để gắn danh
tính vào. Và nó không phải bản nhẹ hơn của hai service kia mà là **tập hợp
lớn hơn**: payload `transaction.risk-scored` chỉ mang `account_ids`, không
mang `client_id`, nên `notification-service` cần đúng bảng chiếu
`account_owners` đó để quy account về client **tại thời điểm `record()`**,
rồi **ghi `client_id` xuống cột riêng** của `notifications` để đường đọc
lọc được bằng bound parameter. Hai service kia chỉ cần bảng chiếu để *kiểm
tra* lúc ghi; service này cần bảng chiếu *cộng thêm* một cột được persist,
vì mỗi lần list là đọc lại lịch sử, không thể quy lại tenant cho từng row
lúc query. Kèm một migration thêm cột và index theo `client_id`.

Cho tới khi spec đó ra đời, endpoint này đọc được chéo tenant — gateway
README phải ghi rõ.

Đây là spec riêng, khối lượng ngang chính gateway. Nó chỉ có nghĩa **sau
khi** đã có client và key, nên thứ tự này là đúng, không phải bỏ sót.

**README của gateway phải nói rõ auth không phải authorization**, để không ai
đọc "đã có API key" rồi tưởng đã an toàn.

### 10.2 Rate limit trong bộ nhớ chỉ đúng với một instance

Chạy 2 replica là ngưỡng thực tế thành gấp đôi. Dùng Redis thì đúng nhưng
thêm một hạ tầng phải vận hành, thuộc phần devops người dùng tự quản. Làm
in-memory và **ghi rõ giới hạn này trong README** thay vì kéo Redis vào lúc
chưa scale.

## 11. Để lại cho việc khác

- **JWT + danh tính người dùng** — §1
- **Phân quyền theo quyền sở hữu account** — §10.1. Thứ tự đã chốt
  2026-09-12: làm **sau khi có CI**, không phải ngay sau gateway. Lý do:
  spec đó đụng vào cả `ledger-service` lẫn `settlement-engine` bằng
  consumer và bảng chiếu mới, tức là sửa đường ghi của sổ cái. Có CI trước
  thì thay đổi đó được máy kiểm chứng thay vì dựa vào việc chạy test thủ
  công — và chính gateway ở spec này cũng được hưởng điều đó.
- **`AUTH-01` vào `docs/BUSINESS_RULES.md`** — chỉ thêm khi gateway đã chạy,
  vì file đó chỉ khẳng định invariant *đã đúng trong code*
- **`securitySchemes` cho 4 file `docs/openapi/*.yaml`** — cùng thời điểm
  trên, cộng một spec cho chính bề mặt công khai của gateway
- **`mobile-app` chuyển sang 1 base URL** — thuộc scope mobile-app
- **Rate limit phân tán bằng Redis** — §10.2
