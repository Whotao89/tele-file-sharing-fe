# Bài báo cáo lần 2
## 1. Giới thiệu
Tài liệu này mô tả quy trình cài đặt và deploy **Telegram File Sharing Bot**, là thành phần **frontend** của hệ thống.  
Bot có nhiệm vụ:
- Nhận file từ người dùng qua Telegram  
- Giao tiếp với backend qua API  
- Hiển thị kết quả, trạng thái xử lý  
- Cung cấp UI dạng hội thoại cho người dùng
  
Bot được phát triển bằng **Golang**, chạy bằng Docker.

---
## 2. Hệ thống đang sử dụng
### 2.1. Phần mềm
| Thành phần | Phiên bản |
|-----------|-----------|
| Go | v1.25 |
| Docker | v28.5.2 |
| Docker Compose | v2.40.3 |
| Git | v2.43.0 |

### 2.2. Phần cứng
- Chưa xác định

---
## 3. Cấu trúc dự án (tính tới hiện tại)
```bash
tele-file-sharing-fe/
├── README.md           # Giới thiệu & hướng dẫn sử dụng
├── Dockerfile          # Docker build đa tầng cho Telegram Bot
├── compose.yml         # Docker Compose (environment for dev)
│
├── .github/
│ └── workflows/
│   └── ci.yml          # CI Pipeline (build-only, chưa có test/lint)
│
├── env/
│ ├── example.env       # File mẫu config (TELEGRAM_BOT_TOKEN, BE_API_BASE,…)
│ └── dev.env (ignored) # File env dành cho localhost/dev (không push)
│
├── cmd/
│ └── bot/
│ └── main.go           # Entry point — chạy bot (polling mode)
│
├── internal/
│ ├── service/          # Business logic — xử lý với backend API
│ │ ├── file_service.go   # Gọi API file, xử lý logic danh sách/tạo file
│ │ ├── share_service.go  # Xử lý logic chia sẻ file
│ │ ├── upload_service.go # Điều phối upload/URL presigned
│ │ └── user_service.go   # Thông tin người dùng (me, profile,…)
│ │
│ ├── config/
│ │ └── config.go       # Load/validate env (BOT_TOKEN, BASE_URL,…)
│ │
│ ├── bot/              # Toàn bộ Telegram Bot UI logic
│ │ ├── handler.go        # Xử lý lệnh chung (/start, /me,…)
│ │ ├── upload_handler.go # Nhận & xử lý upload file
│ │ ├── share_handler.go  # Tạo/xem/revoke share
│ │ ├── keyboards.go      # Inline keyboards, menu buttons
│ │ ├── middleware.go     # Đính kèm API client, User context
│ │ └── router.go         # Định tuyến lệnh
│ │
│ └── api/
│ ├── client.go         # HTTP client gửi request tới backend BE
│ └── models.go         # Struct request/response chuẩn với backend
│
├── docs/               # Tài liệu nội bộ (design, notes,…)
│
└── reports/            # Báo cáo môn học
├── M1/                 # Báo cáo giai đoạn 1 (Phân tích)
├── M2/                 # Báo cáo giai đoạn 2 (Triển khai, cài đặt)
└── M3/                 # Báo cáo giai đoạn 3 (Hoàn thiện, demo)
```

---
## 4. Cấu hình môi trường
- Tạo file: `env/dev.env`
- Nội dung:
```bash
### Telegram Bot ###
TELEGRAM_BOT_TOKEN="YOUR_TOKEN"

### Backend API Base ###
# VD: "https://yourdomain.com"
BE_API_BASE="https://your_backend"

### Webhook Config ###
USE_WEBHOOK=false
WEBHOOK_URL="https://yourdomain.com/webhook"
PORT="YOUR_PORT"

### Optional ###
# Timeout cho request tới backend
API_TIMEOUT=10

# Kích hoạt debug mode (Chưa xài)
DEBUG=false

# Local path để lưu file tạm thời (Chưa xài)
TEMP_DIR="/tmp/bot"

# Nếu bot phải truy cập file server riêng (Chưa xài)
FILE_BASE_URL="https://your_backend/static"
```

- Bảng mô tả các biến môi trường (Environment Variables)

| Biến môi trường         | Ví dụ giá trị                     | Bắt buộc | Mô tả chức năng |
| ----------------------- | --------------------------------- | -------- | --------------- |
| **TELEGRAM_BOT_TOKEN**  | `123456:ABCDEF...`                | ✔️ Yes   | Token Bot Telegram, dùng để xác thực khi gọi Telegram Bot API. |
| **BE_API_BASE**         | `https://your_backend`            | ✔️ Yes   | Base URL của Backend API, bot sẽ gọi để upload file, chia sẻ file, lấy thông tin người dùng,… |
| **USE_WEBHOOK**         | `false`                           | ❌ No    | Nếu `true`, bot dùng Webhook mode. Nếu `false`, bot chạy Polling mode (mặc định). |
| **WEBHOOK_URL**         | `https://yourdomain.com/webhook`  | ❌ No    | URL Webhook để Telegram gửi updates (chỉ dùng khi `USE_WEBHOOK=true`). |
| **PORT**                | `8000`                            | ❌ No    | Port để bot lắng nghe Webhook (Polling không dùng). |
| **API_TIMEOUT**         | `10`                              | ❌ No    | Timeout (giây) cho request từ bot → backend API. Giúp bot không treo khi backend chậm. |
| **DEBUG**               | `false`                           | ❌ No    | Bật/tắt chế độ debug. Nếu `true`, bot sẽ log chi tiết hơn (Hiện tại chưa có). |
| **TEMP_DIR**            | `/tmp/bot`                        | ❌ No    | Thư mục lưu file tạm thời (Hiện tại chưa có). |
| **FILE_BASE_URL**       | `https://your_backend/static`     | ❌ No    | Base URL file server riêng (Hiện tại chưa có). |

---
## 5. Deploy Telegram Bot bằng Docker
### 5.1. Build từ image
- Khởi tạo container và đồng thời chạy main.go
```bash
docker compose build
docker compose up -d
docker logs -f telegram_bot
```
- Kết quả hiển thị nếu chạy thành công
```bash
Tải config thành công
Tạo client mới thành công
Lấy thành công TELEGRAM_BOT_TOKEN trong env
Lấy thành công BE_API_BASE trong env
Lấy thành công USE_WEBHOOK trong env
Lấy thành công PORT trong env
Lấy thành công API_TIMEOUT trong env
Lấy thành công DEBUG trong env
Lấy thành công TEMP_DIR trong env
Tạo bot Telegram thành công
Bắt đầu router Telegram...
```
- Tắt và xóa container nếu không còn xài  
`docker compose down`

---
## 6. Dockerfile (Bot)
Xài chế độ polling (không có webhook) nên không cần expose port
> Thông tin file Dockerfile xem [tại đây](https://github.com/dath-251-thuanle/tele-file-sharing-fe/blob/MILESTONE-M2/Dockerfile)

---
## 7. Docker Compose (Bot)
Xài chế độ polling (không có webhook) nên không cần expose port
> Thông tin file compose.yml xem [tại đây](https://github.com/dath-251-thuanle/tele-file-sharing-fe/blob/MILESTONE-M2/compose.yaml)

---
## 8. Hướng dẫn test bot
- Chạy bot (local): `go run cmd/bot/main.go`
- Chạy lệnh `/start`, `/me`, `myfiles`,... để thử nghiệm
- Kết quả thử nghiệm (tính tới hiện tại):
  - [`/start`](images/start.png)
  - [`/me`](images/me.png)
  - [`/myfiles`](images/myfiles_rỗng.jpg) (khi chưa có file)
  - [`/myfiles`](images/myfiles_chứa.png) (khi có file)
  - [`/upload`](images/upload_HTTP500.png) (internal server error, chưa fix được, cần liên hệ với BE để xử lý sau)
  - [`/share <share_id>`](images/share.png)
  - [`/revoke <share_id>`](images/revoke.png)
