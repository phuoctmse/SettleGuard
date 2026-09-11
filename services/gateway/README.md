# gateway

Reverse proxy mỏng đứng trước 4 backend service. Là chỗ **duy nhất** làm
CORS, xác thực API key cho client business, rate limit và request ID. Không
sở hữu thực thể nghiệp vụ nào, không consume hay publish NATS.

Thiết kế: `docs/superpowers/specs/2026-09-10-gateway-design.md`.

## Định tuyến

| Tiền tố | Upstream (mặc định) |
|---|---|
| `/ledger/*` | `http://localhost:8080` |
| `/accounts/*` | `http://localhost:8081` |
| `/settlement/*` | `http://localhost:8082` |
| `/notifications/*` | `http://localhost:8083` |

Tiền tố bị bỏ trước khi chuyển tiếp: `POST /ledger/transactions` tới
ledger-service là `POST /transactions`. `GET /health` là endpoint duy nhất
không cần key.

## Chạy

```bash
docker compose -f infra/docker/docker-compose.yml up -d gateway-postgres
cd services/gateway
export DATABASE_URL="postgres://gateway:gateway@localhost:5437/gateway?sslmode=disable"
export CORS_ALLOWED_ORIGINS="http://localhost:8090"
go run ./cmd/server
```

Biến môi trường: xem `.env.example`. `CORS_ALLOWED_ORIGINS` rỗng nghĩa là
**chặn mọi origin** — cấu hình thiếu fail-closed.

## Cấp key

Không có endpoint HTTP nào cấp key. Dùng CLI, chạy bởi người có quyền truy
cập DB:

```bash
export DATABASE_URL="postgres://gateway:gateway@localhost:5437/gateway?sslmode=disable"
go run ./cmd/adminctl create --client-id <ClientBusiness uuid> --label "mobile-app prod"
go run ./cmd/adminctl list   --client-id <uuid>
go run ./cmd/adminctl revoke --id <key uuid>
```

Key thô hiện **một lần** lúc tạo; DB chỉ giữ SHA-256. Gọi API bằng
`Authorization: Bearer sg_live_...`.

## Build / lint / test

```bash
go build ./... && go vet ./...
golangci-lint run ./...
go test -count=1 -p 1 ./...     # -p 1: test Docker chạy song song tranh nhau và rớt ngẫu nhiên
```

## Giới hạn đã biết — đọc trước khi tin vào gateway

**Xác thực không phải phân quyền.** Gateway cho biết request đến từ client
nào và gắn `X-Client-Id`, nhưng **không** service nào enforce quyền sở hữu.
Client A vẫn có thể `POST /ledger/transactions` trỏ vào account của client
B, và accounts-service sẽ cập nhật số dư của B. Chỗ sửa là từng service
(bảng chiếu từ `account.updated`), là spec riêng, làm sau CI.

**`GET /notifications/...` đọc được chéo tenant.** Bảng `notifications`
không có cột tenant; endpoint trả `account_ids`, `amount` và
`triggered_rules` của mọi client cho bất kỳ key hợp lệ nào. Sửa cùng spec
phân quyền ở trên.

**Rate limit trong bộ nhớ.** Đúng với một instance. Hai replica là ngưỡng
thực tế gấp đôi. Redis khi cần scale.

**Chưa có JWT / người dùng.** Chỉ có API key cho client business. Danh
tính người dùng mobile/ops là spec riêng.

**Graceful shutdown chưa được xác minh live trên Windows.** Code drain SIGTERM/SIGINT giống ba service kia (PR #18) và đúng khi đọc; nhưng từ Git Bash trên Windows, `kill -INT` không tới được tiến trình Go native nên chưa quan sát được log `shut down cleanly` khi chạy thật. Xác minh trên Linux/WSL hoặc Ctrl+C trong console thật.
