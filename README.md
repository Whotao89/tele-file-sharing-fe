# 📦 Telegram File Sharing — Frontend (Telegram Bot)  
Dự án **Tele File Sharing – Frontend** là Telegram Bot dùng để giao tiếp với người dùng, cho phép upload file, xem danh sách file, chia sẻ file và tương tác với Backend API.  
Bot được viết bằng **Go**, giao tiếp trực tiếp với **Telegram Bot API**, và kết nối đến **tele-file-sharing-be** thông qua HTTP.

---
## 👥 Thành viên nhóm Frontend
| Họ tên                      | Vai trò                                                                                   |
| --------------------------- | ----------------------------------------------------------------------------------------- |
| Nguyễn Nguyên Ngọc (Leader) | Tổng hợp và tạo file sườn, sync data giữa FE và BE, hỗ trợ thành viên, use case (share), viết reports cho M2   |
| Bùi Hoàng Cung              | Làm requirement (non-functional), use case (upload), xử lý request để gửi tới BE  |
| Nguyễn Trí Thành            | Làm requirement (non-functional), use case (revoke), tạo service để xử lý với BE API   |
| Võ Hùng Dũng                | Làm requirement (non-functional), use case (download), tạo router để đưa các lệnh chuyển ngược về hàm xử lý |

---
## 🚀 Hướng dẫn chạy dự án (Local Development)
### 1. Chuẩn bị môi trường
Cần cài đặt trước:  
- [Go](https://go.dev/doc/install) (phiên bản 1.25+)
- [Docker](https://www.docker.com/products/docker-desktop/)
- Docker Compose
- [Sops](https://github.com/getsops/sops) để mã hóa file dev.enc (Hiện tại, nhóm bị trục trặc sops nên chưa có encrypt dev.env thành dev.enc được)

### 2. Cấu hình môi trường
- Tạo file env:
```bash
cp env/example.env env/dev.env
```

- Nếu dùng file .enc:
```bash
sops -d env/dev.enc > env/dev.env
```

- Cấu hình trong env/example.env:
```bash
TELEGRAM_BOT_TOKEN="YOUR_TOKEN"
BE_API_BASE="https://your_backend"

USE_WEBHOOK=false
WEBHOOK_URL="https://yourdomain.com/webhook"
PORT="YOUR_PORT"

API_TIMEOUT=10
DEBUG=false
TEMP_DIR="/tmp/bot"
FILE_BASE_URL="https://your_backend/static"
```

### 3. Chạy bot bằng Docker
```bash
docker compose build
docker compuse up -d
docker logs -f telegram-bot
```

### 4. Chạy bot trực tiếp bằng Go
```bash
go mod tidy
go run ./cmd/bot
```

---
## 🤖 Các lệnh Telegram hỗ trợ
| Lệnh                 | Chức năng                             |
| -------------------- | ------------------------------------- |
| `/start`             | Trợ giúp |
| `/me`                | Xem thông tin người dùng trên backend |
| `/myfiles`             | Liệt kê các file đã upload            |
| `/share <share_id>`       | Tạo link chia sẻ file                 |
| `/upload` (chưa thành công)    | Gửi file, bot tự tải file lên backend |
| (Chưa thử) download  | Tải file về từ đường dẫn được chia sẻ |

---
## 📂 Cấu trúc thư mục
```
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
## 🛠 Quy trình phát triển một tính năng mới
### 1. Tạo nhánh mới
Tạo nhánh riêng biệt trên repository chung và tự gửi Commit & Pull Request để chờ duyệt
```bash
git checkout -b <ten-tinh-nang>
git push origin dev:<ten-tinh-nang>
```

### 2. Cập nhật API (nếu backend thay đổi)
- Chỉnh sửa internal/api/client.go
- Cập nhật struct trong models.go
- Điều chỉnh command tương ứng trong các <feat>_handler.go

### 3. Viết code
- internal/service → Xử lý BE API
- internal/api/* → Giao tiếp BE
- internal/bot/* → Logic command
- internal/config/* → Thêm config nếu cần

### 4. Test thủ công qua Telegram
- Gửi file thử
- Test lệnh /start, /me, /myfiles,...
- Kiểm tra trường hợp backend lỗi

### 5. Tạo Pull Request
- Push code lên nhánh riêng
- Tạo PR vào main
- Mô tả rõ:
  - Mục đích thay đổi
  - Flow backend liên quan
  - Ảnh chụp bot (nếu cần)

### 6. Merge & dọn nhánh
```bash
git push origin --delete <ten-tinh-nang>

```


