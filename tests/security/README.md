# tests/security

Hai loại test:

- **Fraud-bypass** (`test_fraud_bypass.py`): rule nghiệp vụ của settlement-engine.
- **Pentest** (`test_gateway_*.py`, `test_input_hardening.py`, `test_known_gaps.py`, `zap/`): tấn công gateway và 4 service từ bên ngoài. Kết quả tổng hợp ở `docs/security/PENTEST_REPORT.md`.

## Chạy

Cần stack như `tests/README.md`, cộng thêm **hai** gateway (từ `services/gateway`):

    export DATABASE_URL="postgres://gateway:gateway@localhost:5437/gateway?sslmode=disable"
    export CORS_ALLOWED_ORIGINS="http://localhost:8090"
    RATE_LIMIT_IP_PER_MINUTE=100000 RATE_LIMIT_CLIENT_PER_MINUTE=100000 go run ./cmd/server
    # terminal khác, cùng hai export trên:
    LISTEN_ADDR=:8001 RATE_LIMIT_IP_PER_MINUTE=20 RATE_LIMIT_CLIENT_PER_MINUTE=10 go run ./cmd/server

Vì sao hai instance: ZAP và các test khác bắn từ cùng một IP, nên với ngưỡng
thật (60/phút) mọi thứ sẽ bị 429. Instance `:8001` để riêng cho test rate limit.

    cd tests && python -m pytest security -v -rxX

Các lệnh ZAP dưới đây chạy từ **thư mục gốc repo** (không phải `tests/`):

    bash tests/security/zap/run_zap.sh      # Linux/WSL
    ZAP_GATEWAY_URL=http://host.docker.internal:8000 ZAP_DOCKER_NETWORK=bridge bash tests/security/zap/run_zap.sh   # Windows (Git Bash, Docker Desktop)

Key được cấp bằng `adminctl` thật. Đặt `GATEWAY_ADMINCTL` trỏ tới binary đã
build để khỏi `go run` mỗi lần.

Log của từng lần quét ZAP được giữ ở `tests/security/zap/out/<spec>.log`. Log
này là bằng chứng rằng active scan đã chạy.

## Đọc kết quả

- `XFAIL`: lỗ hổng được xác nhận (finding vẫn còn).
- `FAILED` (đỏ): lỗi dựng điều kiện tiên quyết (setup), không phải finding.
- `SKIP`: stack chưa chạy hoặc service không với tới được.

### `xfail` là finding, không phải lỗi test

Mỗi lỗ hổng là một test `xfail(strict=True, reason="SG-PT-NN: ...")` khẳng
định hành vi **an toàn**. Các finding SG-PT-01/02/03/06/08 dùng
`xfail(strict=True, raises=AssertionError)`: chỉ một assertion thất bại mới
tính là XFAIL; điều kiện tiên quyết hỏng sẽ gọi `pytest.fail` và làm suite đỏ.
Khi lỗ hổng được sửa, test XPASS và `strict` làm suite đỏ. Lúc đó, gỡ marker
và cập nhật trạng thái finding trong báo cáo, trong cùng một PR.

## Test phá hủy (destructive)

`test_negative_notification_limit_is_refused` được đánh dấu `destructive` và
`tests/pyproject.toml` loại nó khỏi lần chạy mặc định (`-m "not destructive"`).
Test này có thể khiến notification-service không phục vụ được `/notifications`
nữa. Chỉ chạy nó **cuối cùng và riêng một mình**:

    cd tests && python -m pytest security -m destructive

Sau đó phải khởi động lại notification-service. Trong CI, nó chạy ở bước cuối
của job `security`, sau ZAP.

## Tác dụng phụ

Suite ghi dữ liệu thật: ví dụ SG-PT-08 lưu một tên client dài 5 MiB, SG-PT-01/02
tạo client và giao dịch ledger. Chỉ chạy trên stack local dùng một lần, **không
bao giờ** chạy trên môi trường dùng chung.

## Rate limit

Các test rate limit cần bucket của `:8001` còn đầy. Chờ khoảng 60 giây giữa hai
lần chạy lại file `test_gateway_rate_limit.py`.

## ZAP

`zap/check_report.py` fail khi có alert **High** chưa nằm trong
`zap/accepted_risks.yaml`. Mỗi mục chấp nhận phải có `plugin_id`,
`finding_id` và `reason`. Alert Medium chỉ được báo cáo, không làm fail.
`check_report.py` cũng fail khi lần quét không tạo ra kết quả nào (fail closed),
nên một lần quét hỏng không thể bị đọc nhầm là sạch.
