# Tài liệu tham khảo: Thuật ngữ (Wording)

Tra cứu nhanh các thuật ngữ hay gặp khi trao đổi về SettleGuard — cả thuật
ngữ nghiệp vụ (business) lẫn thuật ngữ kỹ thuật/kiến trúc. Không cần đọc hết
một lượt, quên thuật ngữ nào thì mở ra tra thuật ngữ đó.

Cùng nhóm với `docs/SYNTAX_REFERENCE.md` (tra cú pháp code) nhưng file này
tra **khái niệm**, không tra cú pháp.

## Phần 1 — Thuật ngữ nghiệp vụ (business/domain)

| Thuật ngữ | Giải thích dễ hiểu | Trong SettleGuard |
|---|---|---|
| **Obligation** (nghĩa vụ thanh toán) | Một bên "nợ" bên kia một khoản tiền/giá trị, chưa chắc đã trả. Không phải tiền thật đã chuyển. | SettleGuard theo dõi các nghĩa vụ này giữa các party, chứ không tự chuyển tiền. |
| **Settlement** (quyết toán / tất toán) | Hành động "chốt sổ" — gom các nghĩa vụ đã được duyệt (không có rủi ro) lại thành một đợt xử lý, coi như đã xong. | `settlement-engine` chạy theo lịch, gom các ledger entry đã risk-clear thành 1 settlement batch. |
| **Ledger** (sổ cái) | Nơi ghi lại mọi giao dịch/bút toán, không được sửa/xóa sau khi ghi (append-only). Là "nguồn sự thật" — mọi service khác đều dựa vào đây. | `ledger-service` sở hữu ledger, publish event `ledger.entry-recorded` mỗi khi có bút toán mới. |
| **Ledger entry** (bút toán) | Một dòng ghi trong ledger — một lần ghi nhận nghĩa vụ/thay đổi. | Mỗi giao dịch tạo ra ít nhất 1 ledger entry. |
| **Double-entry** (bút toán kép) | Nguyên tắc kế toán: mỗi giao dịch ghi 2 lần — một bên ghi Nợ, một bên ghi Có, tổng luôn cân bằng. Giúp tự phát hiện sai lệch. | Ledger của SettleGuard thiết kế theo kiểu double-entry để đảm bảo số liệu luôn khớp. |
| **Risk score / Risk scoring** (chấm điểm rủi ro) | Gán một điểm số (hoặc nhãn) cho một giao dịch để đánh giá khả năng gian lận/bất thường, dựa trên rule (ngưỡng cố định) + ML (mô hình học máy). | `settlement-engine` chấm điểm rủi ro real-time cho từng giao dịch trước khi cho vào settlement. |
| **Account** (tài khoản) | Khái niệm nội bộ của SettleGuard — **không phải** tài khoản ngân hàng thật. Đại diện cho một đối tượng (người dùng cuối) có nghĩa vụ được theo dõi. | `accounts-service` sở hữu dữ liệu Account. |
| **Client business** | Doanh nghiệp khách hàng tích hợp API của SettleGuard (B2B) — ví dụ một app ví điện tử dùng SettleGuard để quản lý settlement giữa người dùng của họ. | Mỗi `Account` thuộc về một `ClientBusiness`. |
| **Party** (bên liên quan) | Thuật ngữ gộp chung cho "một bên tham gia giao dịch" — có thể là Account hoặc ClientBusiness. Hiện SettleGuard chưa có type `Party` thống nhất (còn để riêng 2 type), đây là ý tưởng gộp trong tương lai. | Nhắc tới trong charter/design doc như một khái niệm domain, chưa implement. |
| **Balance-of-obligation** (số dư nghĩa vụ) | Tổng nghĩa vụ hiện tại của một Account, tính từ các ledger entry liên quan. Không phải số dư tiền thật. | `accounts-service` sẽ tính cái này sau khi consume được event `ledger.entry-recorded` — hiện **chưa** làm vì chưa có event broker. |
| **Alert / Notification** (cảnh báo) | Thông báo gửi cho end-user hoặc ops team khi có rủi ro hoặc settlement xong. | `notification-service` gửi qua push/email, chỉ lắng nghe event, không ai gọi trực tiếp nó. |

## Phần 2 — Thuật ngữ kiến trúc/kỹ thuật (architecture)

| Thuật ngữ | Giải thích dễ hiểu | Ví dụ / vì sao quan trọng |
|---|---|---|
| **Event** | Một "sự kiện đã xảy ra" được ghi lại dưới dạng dữ liệu, để service khác biết mà phản ứng. Khác với gọi API trực tiếp — bên phát event không cần biết ai đang nghe. | `ledger.entry-recorded`, `account.updated` là các event. |
| **Event-driven** | Kiến trúc mà các service giao tiếp chủ yếu qua event thay vì gọi API lẫn nhau trực tiếp. | Toàn bộ SettleGuard thiết kế theo kiểu này — không có service nào gọi thẳng service khác. |
| **Event broker / Message broker** | Một hệ thống trung gian đứng giữa: bên gửi event bỏ vào broker, bên nhận tự lấy ra từ broker — hai bên không cần biết nhau. | Đây chính là mảnh còn thiếu — công nghệ cụ thể (NATS/Kafka/RabbitMQ...) chưa được chọn. |
| **Publish / Subscribe (pub/sub)** | "Publish" = gửi event vào broker. "Subscribe" = đăng ký nhận event từ broker. | `accounts-service` publish `account.updated`; `settlement-engine` subscribe để nhận. |
| **Topic / Stream / Queue** | Một "kênh" đặt tên trong broker mà event được gửi vào/lấy ra. Topic/Stream thường cho phép nhiều người đọc độc lập (đọc xong không mất); Queue theo kiểu cổ điển thường 1 event chỉ được 1 consumer lấy rồi mất. | Mỗi loại event (`ledger.entry-recorded`, `settlement.finalized`...) sẽ có 1 topic/stream riêng. |
| **Consumer / Consumer group** | Consumer = bên đọc event từ broker. Consumer group = một nhóm consumer chia nhau đọc, hoặc mỗi group đọc độc lập không ảnh hưởng group khác. | `settlement-engine` và `notification-service` là 2 consumer group khác nhau, đọc độc lập cùng một dòng event. |
| **Durability** (tính bền) | Event đã ghi vào broker thì không bị mất, kể cả khi broker restart hay consumer đang down. | Bắt buộc phải có cho event tài chính như `ledger.entry-recorded` — mất là mất dữ liệu thật. |
| **Ordering** (thứ tự) | Event tới consumer đúng theo thứ tự nó được publish (ít nhất là trong cùng 1 "khóa", ví dụ cùng 1 account). | Nếu 2 bút toán của cùng 1 account tới sai thứ tự, risk scoring có thể tính sai. |
| **Replay** (phát lại) | Khả năng đọc lại các event cũ đã publish từ trước (không chỉ nhận event mới). | Hữu ích khi sửa lại risk model rồi muốn chạy lại trên dữ liệu cũ để so sánh, hoặc điều tra gian lận. |
| **At-least-once delivery** | Cam kết của broker: event sẽ tới consumer **ít nhất** 1 lần — nhưng có thể bị gửi trùng (2 lần) trong một số trường hợp lỗi mạng. | Vì vậy consumer phải tự chịu được việc nhận trùng — xem **Idempotency**. |
| **Idempotency / Idempotent** | Tính chất: xử lý cùng một event 2 lần cho ra kết quả giống hệt xử lý 1 lần (không bị cộng dồn/tính 2 lần). | Cần thiết vì broker có thể gửi trùng event (at-least-once) — service nhận phải tự kiểm tra tránh xử lý lại. |
| **Outbox pattern** | Kỹ thuật đảm bảo: ghi dữ liệu vào DB và "chuẩn bị gửi event" xảy ra cùng lúc, chắc chắn không bị lệch (ví dụ ghi ledger entry xong nhưng crash trước khi gửi event). | Kế hoạch dùng outbox pattern khi implement publisher cho `ledger-service`/`accounts-service`. |
| **Log-based broker** (vd. Kafka, NATS JetStream) | Event được lưu lại thành một "nhật ký" tuần tự, consumer đọc bằng cách di chuyển một con trỏ (offset) trên nhật ký đó — đọc xong event vẫn còn nguyên, đọc lại được. | Phù hợp với nhu cầu replay/audit của ledger tài chính. |
| **Queue-based broker** (vd. RabbitMQ kiểu truyền thống) | Event nằm trong hàng đợi, một consumer lấy ra là event biến mất khỏi hàng đợi đó. | Muốn nhiều consumer độc lập cùng đọc 1 event thì phải tự dựng thêm cơ chế nhân bản (fanout). |
| **Partition / Offset** | Partition = một topic được chia nhỏ thành nhiều "làn" song song để tăng throughput. Offset = vị trí/con trỏ đánh dấu consumer đã đọc tới đâu trong 1 partition. | Thuật ngữ đặc trưng của Kafka — một phần lý do Kafka phức tạp hơn để vận hành. |

## Phần 3 — Thuật ngữ quy trình dự án

| Thuật ngữ | Giải thích dễ hiểu |
|---|---|
| **MVP** (Minimum Viable Product) | Bản làm đủ dùng, đủ đúng để chạy thật — không phải bản đầy đủ tính năng. |
| **Migration** (schema migration) | File `.sql` mô tả một thay đổi cấu trúc database (tạo bảng, thêm cột...), chạy tuần tự để đưa DB từ trạng thái cũ sang mới. |
| **Testcontainers** | Thư viện tự động bật một Postgres thật (trong Docker) chỉ để chạy test, tắt đi sau khi test xong — test không phải giả lập (mock) DB. |
| **Mentor mode** | Cách làm việc đã thống nhất trong project: giải thích trước, để bạn tự viết code, không paste sẵn code vào file. |
