# tests/e2e

Bộ E2E bằng Python Playwright chạy trên **bản web build** của `mobile-app`,
đi qua gateway thật. Thiết kế: `docs/superpowers/specs/2026-10-18-pentest-and-playwright-design.md` §7.

## Cách nó hoạt động

- Bản web được build với các `EXPO_PUBLIC_*_API_URL` trỏ vào gateway
  (`http://localhost:8000/<prefix>`) và serve ở `http://localhost:8090` —
  đúng origin mà gateway cho phép CORS.
- App không gửi API key. Playwright gắn key vào mọi request bằng
  `extra_http_headers` của browser context, nên key không bao giờ nằm trong
  bundle. Key được cấp bằng `adminctl` thật lúc bắt đầu session.
- Dữ liệu được seed qua API (cổng trực tiếp của service); UI chỉ dùng để xem
  và thao tác. Selector là `testID` (trên web là `data-testid`), xem bảng
  trong `mobile-app/README.md`.

## Chạy local

Cần stack như `tests/README.md` (4 service, NATS, Postgres) và gateway `:8000`
khởi động với `CORS_ALLOWED_ORIGINS=http://localhost:8090` — để trống thì
gateway chặn mọi origin và mọi test sẽ thấy trạng thái lỗi.

    pip install -r tests/requirements.txt
    python -m playwright install chromium

Build và serve bản web (từ repo root, terminal riêng):

    cd mobile-app
    EXPO_PUBLIC_ACCOUNTS_API_URL=http://localhost:8000/accounts \
    EXPO_PUBLIC_LEDGER_API_URL=http://localhost:8000/ledger \
    EXPO_PUBLIC_SETTLEMENT_API_URL=http://localhost:8000/settlement \
    EXPO_PUBLIC_NOTIFICATION_API_URL=http://localhost:8000/notifications \
      npx expo export --platform web --output-dir ../.e2e-web
    cd .. && python -m http.server 8090 -d .e2e-web

Biến môi trường ở trên ghi đè `mobile-app/.env`. Build lại mỗi khi code app đổi.

Chạy test (từ `tests/`):

    python -m pytest e2e -v --browser chromium
    python -m pytest e2e -v --browser chromium --headed      # xem trình duyệt
    python -m pytest e2e --tracing on                        # luôn giữ trace

Xem trace: `python -m playwright show-trace test-results/<tên test>/trace.zip`.

## Đọc kết quả

- **SKIPPED**: gateway hoặc bản web không chạy.
- Trạng thái lỗi của app (`"Failed to load …"`) chỉ hiện sau ~7,5 s vì
  react-query thử lại 3 lần; test về key bị revoke chờ tới 20 s.
- Tab Held và Settlements không tự tải lại: mỗi test seed dữ liệu **trước**
  rồi mới mở tab lần đầu. Nếu tự viết test mới, giữ đúng thứ tự đó hoặc
  `page.reload()`.
- Danh sách Held và Settlements phân trang (mới nhất trước, 50 dòng/trang);
  test chỉ tìm ở trang đầu, nên đừng chạy song song nhiều suite ghi dữ liệu
  cùng lúc.
- `/notifications` không trả lời → có thể test `destructive` của
  `tests/security` (SG-PT-07) vừa chạy; khởi động lại notification-service.

## CI

Job `e2e-web` trong `.github/workflows/ci.yml` dựng stack, build và serve bản
web, chạy suite, rồi upload artifact `e2e-web-report`: `e2e-report.html`
(pytest-html) và `test-results/` (trace, video, screenshot của test fail). Mở
trace đã tải về bằng `python -m playwright show-trace <file>` hoặc
https://trace.playwright.dev.

Playwright cho Python không có HTML reporter như bản Node; `pytest-html` cùng
trace là phần thay thế.
