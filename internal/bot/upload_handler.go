package bot

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"fe-file-sharing/internal/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// User states cho Share Wizard
const (
	StateNone            = "none"
	StateAwaitingPassword = "awaiting_password"
	StateAwaitingExpiresAt = "awaiting_expires_at"
	StateAwaitingConfirm = "awaiting_confirm"
)

type BotHandler struct {
    TG         *tgbotapi.BotAPI
    BotUsername string
    UploadSvc  *service.UploadService
    ShareSvc   *service.ShareService
    FileSvc    *service.FileService
    UserSvc    *service.UserService
    
    // States: user state -> data (fileID, password, expires_at)
    StateMu    sync.RWMutex
    States     map[int64]map[string]interface{} // telegramID -> {state: "...", fileID: 123, password: "...", ...}
}

func NewBotHandler(tg *tgbotapi.BotAPI, us *service.UploadService, ss *service.ShareService, fs *service.FileService, usvc *service.UserService) *BotHandler {
    return &BotHandler{
        TG: tg,
        BotUsername: tg.Self.UserName,
        UploadSvc: us,
        ShareSvc: ss,
        FileSvc: fs,
        UserSvc: usvc,
        States: make(map[int64]map[string]interface{}),
    }
}

// HandleUpload processes incoming document messages and uses services
func (h *BotHandler) HandleUpload(update tgbotapi.Update) {
    doc := update.Message.Document
    chatID := update.Message.Chat.ID
    user := update.Message.From

    // Download file from Telegram
    file, err := h.TG.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
    if err != nil {
        h.replyRaw(chatID, "Không tải được file từ Telegram: "+err.Error())
        return
    }

    url := file.Link(h.TG.Token)
    resp, err := http.Get(url)
    if err != nil {
        h.replyRaw(chatID, "Lỗi khi tải file: "+err.Error())
        return
    }
    defer resp.Body.Close()

    // Validate size
    if err := h.FileSvc.ValidateSize(int64(doc.FileSize), 100*1024*1024); err != nil {
        h.replyRaw(chatID, "File invalid: "+err.Error())
        return
    }

    // Save to temp
    tmpPath, err := h.FileSvc.SaveTempFile(resp.Body, doc.FileName)
    if err != nil {
        h.replyRaw(chatID, "Lỗi lưu file tạm: "+err.Error())
        return
    }
    defer h.FileSvc.CleanUp(tmpPath)

    // Start upload: get fileID and presigned URL
    fileID, uploadURL, err := h.UploadSvc.StartUpload(int64(user.ID), doc.FileID, doc.FileName, int64(doc.FileSize), doc.MimeType)
    if err != nil {
        h.replyRaw(chatID, "Không thể khởi tạo upload: "+err.Error())
        return
    }

    // Get actual file size
    fileInfo, _ := os.Stat(tmpPath)
    actualSize := int64(0)
    if fileInfo != nil {
        actualSize = fileInfo.Size()
    }

    // Track upload time
    startTime := time.Now()

    // Upload to MinIO (presigned PUT)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()
    if err := h.UploadSvc.UploadToMinIO(ctx, uploadURL, tmpPath); err != nil {
        h.replyRaw(chatID, "Upload thất bại: "+err.Error())
        return
    }

    // Calculate metrics
    uploadDuration := time.Since(startTime)
    durationMs := uploadDuration.Milliseconds()
    bandwidthKbps := float64(0)
    if durationMs > 0 {
        // bandwidth in KB/s = (bytes / 1024) / (ms / 1000)
        bandwidthKbps = (float64(actualSize) / 1024.0) / (float64(durationMs) / 1000.0)
    }

    // Complete upload with metrics
    err = h.UploadSvc.CompleteUpload(int64(user.ID), fileID, actualSize, durationMs, bandwidthKbps)
    if err != nil {
        h.replyRaw(chatID, "Không thể xác nhận hoàn tất upload: "+err.Error())
        return
    }

    // Send success with inline buttons: myfiles and share
    successText := "✅ File hợp lệ! Tải lên thành công.\nBạn muốn làm gì tiếp theo?"
    kb := PostUploadInline(fileID)
    msg := tgbotapi.NewMessage(chatID, successText)
    msg.ReplyMarkup = kb
    h.TG.Send(msg)
}

func (h *BotHandler) HandleMe(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    user := update.Message.From

    name := user.UserName
    if name == "" {
        name = user.FirstName

        if user.LastName != "" {
            name = user.FirstName + " " + user.LastName
        }
    }

    u, err := h.UserSvc.GetMe(int64(user.ID), name)
    if err != nil {
        h.replyRaw(chatID, "Lỗi lấy thông tin người dùng: "+err.Error())
        return
    }

    created := u.CreatedAt.In(time.Local).Format("02/01/2006 15:04:05")

    msg := fmt.Sprintf("👤 *THÔNG TIN TÀI KHOẢN*\n\n🆔 Hệ thống: `%d`\n🆔 Telegram: `%d`\n👤 Username: `@%s`\n📅 Ngày tạo: %s", 
        u.ID, u.TelegramID, u.Username, created)

    message := tgbotapi.NewMessage(chatID, msg)
    message.ParseMode = "Markdown"
    h.TG.Send(message)
}

func (h *BotHandler) HandleFiles(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    user := update.Message.From

    files, err := h.FileSvc.ListFiles(int64(user.ID))
    if err != nil {
        h.replyRaw(chatID, "Lỗi lấy danh sách file: "+err.Error())
        return
    }

    if len(files) == 0 {
        h.replyRaw(chatID, "Bạn chưa có file nào.")
        return
    }

    // Send one message per file with inline buttons (View | Share)
    for _, f := range files {
        // friendly size
        sizeKB := f.Size / 1024
        sizeText := fmt.Sprintf("%d KB", sizeKB)
        if sizeKB > 1024 {
            sizeText = fmt.Sprintf("%.1f MB", float64(f.Size)/(1024.0*1024.0))
        }

        text := fmt.Sprintf("%s\nID: #%d · Size: %s", f.Filename, f.ID, sizeText)

        // Dùng callback button để download file (không dùng URL)
        btnView := tgbotapi.NewInlineKeyboardButtonData("🔍 View", fmt.Sprintf("file:view:%d", f.ID))
        btnShare := tgbotapi.NewInlineKeyboardButtonData("🔗 Share", fmt.Sprintf("cmd:share:%d", f.ID))
        btnReport := tgbotapi.NewInlineKeyboardButtonData("📊 Report", fmt.Sprintf("file:report:%d", f.ID))
        row := tgbotapi.NewInlineKeyboardRow(btnView, btnReport, btnShare)
        kb := tgbotapi.NewInlineKeyboardMarkup(row)

        msg := tgbotapi.NewMessage(chatID, text)
        msg.ReplyMarkup = kb
        h.TG.Send(msg)
    }
}

// HandleStart shows the main menu keyboard
func (h *BotHandler) HandleStart(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID

    args := update.Message.CommandArguments() // Lấy phần sau chữ /start

    //Deep Link Tải File (Link có dạng start=share_123)
    if strings.HasPrefix(args, "share_") {
        shareIdentity := strings.TrimPrefix(args, "share_")

        shareID, err := strconv.ParseInt(shareIdentity, 10, 64)
        if err != nil {
            h.replyRaw(chatID, "❌ Link chia sẻ không hợp lệ.")
            return
        }

        meta, err := h.ShareSvc.GetShareMetadata(int64(update.Message.From.ID), shareID)
        
        if err != nil {
            h.replyRaw(chatID, "⚠️ Link chia sẻ không hợp lệ hoặc đã hết hạn.")
            return
        }

        if meta.Revoked {
            h.replyRaw(chatID, "⛔ Link chia sẻ này đã bị thu hồi bởi chủ sở hữu.")
            return
        }

        // Kiểm tra xem share có hết hạn không
        if meta.ExpiresAt != nil && meta.ExpiresAt.Before(time.Now()) {
            h.replyRaw(chatID, fmt.Sprintf("⏰ Link chia sẻ này đã hết hạn vào lúc %s.", meta.ExpiresAt.In(time.Local).Format("02/01/2006 15:04:05")))
            return
        }

        fileInfo := fmt.Sprintf("🎁 *Bạn nhận được một file chia sẻ!*\n\n📄 Tên: `%s`\n📦 Size: %d bytes", 
            meta.File.Filename, meta.File.Size)
        
        msg := tgbotapi.NewMessage(chatID, fileInfo)
        msg.ParseMode = "Markdown"

        // Hiển thị cả nút Tải xuống (không token) và Nhập mật khẩu
        btnDownload := tgbotapi.NewInlineKeyboardButtonData("⬇️ Tải xuống", fmt.Sprintf("cmd:download:%d", shareID))
        btnAuth := tgbotapi.NewInlineKeyboardButtonData("🔐 Nhập Mật Khẩu", fmt.Sprintf("cmd:auth:%d", shareID))
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnDownload, btnAuth))
        
        h.TG.Send(msg)
        return
    }       

    welcome := "Xin chào! Tôi là FileShareBot.\nDưới đây là các lệnh bạn có thể dùng:\n" +
        "\n• /me — Xem thông tin tài khoản của bạn.\n" +
		"• /upload — Tải lên một file: gửi file dưới dạng Document trong chat (không gửi ảnh).\n" +
        "• /myfiles — Xem danh sách file bạn đã tải lên.\n" +
        "• /uploadhistory — Xem lịch sử upload chi tiết.\n" +
        "• /share <file\\_ID> — Tạo link chia sẻ cho một file.\n" +
        "• /myshares — Xem danh sách các link chia sẻ bạn đã tạo.\n" +
        "• /revoke <share\\_ID> — Thu hồi một link chia sẻ.\n\n" 

    msg := tgbotapi.NewMessage(chatID, welcome)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = MainMenuKeyboard()
    h.TG.Send(msg)
}

// HandleCallbackQuery processes inline button presses (callback data)
func (h *BotHandler) HandleCallbackQuery(q *tgbotapi.CallbackQuery) {
    // Acknowledge callback to remove the loading state in client
    cb := tgbotapi.NewCallback(q.ID, "")
    h.TG.Request(cb)

    data := q.Data
    userID := int64(q.From.ID)
    chatID := int64(0)
    if q.Message != nil {
        chatID = q.Message.Chat.ID
    } else {
        // no message (rare), try to send to user
        chatID = int64(q.From.ID)
    }

    switch {
    case data == "cmd:upload":
        h.replyRaw(chatID, "Vui lòng gửi file dưới dạng Document (không gửi ảnh). Tôi sẽ upload giúp bạn.")
    case strings.HasPrefix(data, "cmd:share:"):
        parts := strings.Split(data, ":")
		if len(parts) != 3 {
			h.replyRaw(chatID, "❌ Dữ liệu nút bấm lỗi.")
			return
		}

		fileID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			h.replyRaw(chatID, "❌ ID file không hợp lệ.")
			return
		}

		h.StateMu.Lock()
		if h.States[userID] == nil {
			h.States[userID] = make(map[string]interface{})
		}
		h.States[userID]["fileID"] = fileID
		h.States[userID]["state"] = StateAwaitingPassword
		h.StateMu.Unlock()

		h.replyRaw(chatID, "🔐 Vui lòng nhập mật khẩu để tạo chia sẻ hoặc gõ 'skip' nếu không đặt mật khẩu:")
        
    // Case: Người nhận bấm nút "Tải xuống" (thử tải không kèm token)
    case strings.HasPrefix(data, "cmd:download:"):
        parts := strings.Split(data, ":")
        shareID, _ := strconv.ParseInt(parts[2], 10, 64)

        h.replyRaw(chatID, "⬇️ Đang chuẩn bị tải xuống...")

        // Thử download mà không kèm token
        dataBytes, filename, err := h.ShareSvc.DownloadShare(userID, shareID, "")
        if err != nil {
            errMsg := err.Error()
            // Nếu BE báo cần mật khẩu
            if strings.Contains(strings.ToLower(errMsg), "password_required") ||
               strings.Contains(strings.ToLower(errMsg), "requires password") ||
               strings.Contains(errMsg, "403") {
                // Lưu state để yêu cầu mật khẩu
                h.StateMu.Lock()
                if h.States[userID] == nil { h.States[userID] = make(map[string]interface{}) }
                h.States[userID]["auth_share_id"] = shareID
                h.States[userID]["state"] = "awaiting_auth_password"
                h.StateMu.Unlock()

                h.replyRaw(chatID, "🔐 Share yêu cầu mật khẩu. Vui lòng nhập mật khẩu:")
                return
            }
            // Các lỗi khác
            h.replyRaw(chatID, "❌ Không thể tải xuống: "+errMsg)
            return
        }

        // Tải thành công (share không cần mật khẩu)
        doc := tgbotapi.FileBytes{Name: filename, Bytes: dataBytes}
        h.TG.Send(tgbotapi.NewDocument(chatID, doc))
        return

        // Case: Người nhận bấm nút "Nhập mật khẩu"
        case strings.HasPrefix(data, "cmd:auth:"):
            parts := strings.Split(data, ":")
            shareID, _ := strconv.ParseInt(parts[2], 10, 64)
            
            // Lưu state để đợi nhập pass
            h.StateMu.Lock()
            if h.States[userID] == nil { h.States[userID] = make(map[string]interface{}) }
            h.States[userID]["auth_share_id"] = shareID
            h.States[userID]["state"] = "awaiting_auth_password" // Tạo state mới
            h.StateMu.Unlock()
            
            h.replyRaw(chatID, "🔑 Nhập mật khẩu:")
        
        // Case: File owner bấm nút "Mở" từ /myfiles
        case strings.HasPrefix(data, "file:view:"):
            parts := strings.Split(data, ":")
            fileID, err := strconv.ParseInt(parts[2], 10, 64)
            if err != nil {
                h.replyRaw(chatID, "❌ ID file không hợp lệ.")
                return
            }
            
            h.replyRaw(chatID, "⬇️ Đang lấy file từ server...")
            
            // Tạo share tạm để download (không password, không expire)
            share, err := h.ShareSvc.CreateShare(userID, fileID, "", nil)
            if err != nil {
                h.replyRaw(chatID, "❌ Lỗi tạo link tải: "+err.Error())
                return
            }
            
            // Download qua share (giống như người nhận)
            fileData, filename, err := h.ShareSvc.DownloadShare(userID, share.ID, "")
            if err != nil {
                h.replyRaw(chatID, "❌ Lỗi tải file: "+err.Error())
                return
            }
            
            doc := tgbotapi.FileBytes{Name: filename, Bytes: fileData}
            h.TG.Send(tgbotapi.NewDocument(chatID, doc))

        // Case: File owner bấm nút "Report" từ /myfiles
        case strings.HasPrefix(data, "file:report:"):
            parts := strings.Split(data, ":")
            fileID, err := strconv.ParseInt(parts[2], 10, 64)
            if err != nil {
                h.replyRaw(chatID, "❌ ID file không hợp lệ.")
                return
            }

            h.replyRaw(chatID, "⏳ Đang tải thông tin upload report...")

            report, err := h.FileSvc.GetFileReport(userID, fileID)
            if err != nil {
                h.replyRaw(chatID, "❌ Lỗi khi tải report: "+err.Error())
                return
            }

            // Format report details
            statusIcon := "✅"
            if report.Status != "success" && report.Status != "completed" {
                statusIcon = "❌"
            }

            // Format file size
            sizeText := fmt.Sprintf("%.2f KB", float64(report.FileSizeActual)/1024.0)
            if report.FileSizeActual > 1024*1024 {
                sizeText = fmt.Sprintf("%.2f MB", float64(report.FileSizeActual)/(1024.0*1024.0))
            }

            // Format upload duration
            durationText := fmt.Sprintf("%.2f giây", float64(report.UploadDurationMs)/1000.0)

            // Format bandwidth
            bandwidthText := fmt.Sprintf("%.2f KB/s", report.BandwidthKbps)
            if report.BandwidthKbps > 1024 {
                bandwidthText = fmt.Sprintf("%.2f MB/s", report.BandwidthKbps/1024.0)
            }

            reportText := fmt.Sprintf(
                "%s *UPLOAD REPORT*\n\n"+
                    "🆔 Report ID: `%d`\n"+
                    "📄 File ID: `%d`\n"+
                    "📊 Status: %s\n"+
                    "📦 Size: %s\n"+
                    "⏱️ Duration: %s\n"+
                    "🚀 Speed: %s\n"+
				"📅 Date: %s",
                statusIcon,
                report.ID,
                report.FileID,
                report.Status,
                sizeText,
                durationText,
                bandwidthText,
				report.ReportedAt.In(time.Local).Format("02/01/2006 15:04:05"),
            )

            if report.FileChecksum != "" {
                reportText += fmt.Sprintf("\n🔐 Checksum: `%s`", report.FileChecksum)
            }

            if report.Message != "" {
                reportText += "\n💬 " + report.Message
            }

            if report.ErrorMessage != "" {
                reportText += "\n⚠️ Error: " + report.ErrorMessage
            }

            msg := tgbotapi.NewMessage(chatID, reportText)
            msg.ParseMode = "Markdown"
            h.TG.Send(msg)

        	// ------------------------------
        default:
            h.replyRaw(chatID, "Hành động chưa được hỗ trợ: "+data)
        }
}

// HandleUploadCommand instructs user to send a file/document
func (h *BotHandler) HandleUploadCommand(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    h.replyRaw(chatID, "Vui lòng gửi file dưới dạng Document (nhấn paperclip -> Document) để bot upload.")
}

// HandleUploadHistory hiển thị lịch sử upload chi tiết
func (h *BotHandler) HandleUploadHistory(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	userID := int64(update.Message.From.ID)

	h.replyRaw(chatID, "⏳ Đang tải lịch sử upload...")

	reports, err := h.FileSvc.GetUploadReports(userID, 20, 0)
	if err != nil {
		h.replyRaw(chatID, "❌ Lỗi khi tải lịch sử: "+err.Error())
		return
	}

	if len(reports) == 0 {
		h.replyRaw(chatID, "📋 Bạn chưa có lịch sử upload nào.")
		return
	}

	h.replyRaw(chatID, fmt.Sprintf("📋 Lịch sử upload (%d báo cáo gần nhất):", len(reports)))

	for _, r := range reports {
		// Format status icon
		statusIcon := "✅"
		if r.Status != "success" {
			statusIcon = "❌"
		}

		// Format file size
		sizeText := fmt.Sprintf("%.2f KB", float64(r.FileSizeActual)/1024.0)
		if r.FileSizeActual > 1024*1024 {
			sizeText = fmt.Sprintf("%.2f MB", float64(r.FileSizeActual)/(1024.0*1024.0))
		}

		// Format upload duration
		durationText := fmt.Sprintf("%.2f giây", float64(r.UploadDurationMs)/1000.0)

		// Format bandwidth
		bandwidthText := fmt.Sprintf("%.2f KB/s", r.BandwidthKbps)
		if r.BandwidthKbps > 1024 {
			bandwidthText = fmt.Sprintf("%.2f MB/s", r.BandwidthKbps/1024.0)
		}

		text := fmt.Sprintf(
			"%s Report #%d\n"+
				"📄 File ID: %d\n"+
				"📊 Status: %s\n"+
				"📦 Size: %s\n"+
				"⏱️ Duration: %s\n"+
				"🚀 Speed: %s\n"+
            "📅 Date: %s",
			statusIcon,
			r.ID,
			r.FileID,
			r.Status,
			sizeText,
			durationText,
			bandwidthText,
            r.ReportedAt.In(time.Local).Format("02/01/2006 15:04:05"),
		)

		if r.Message != "" {
			text += "\n💬 " + r.Message
		}

		if r.ErrorMessage != "" {
			text += "\n⚠️ Error: " + r.ErrorMessage
		}

		h.replyRaw(chatID, text)
	}
}

func (h *BotHandler) replyRaw(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    h.TG.Send(msg)
}

// HandleTextInput xử lý tin nhắn văn bản thường dựa trên trạng thái user
func (h *BotHandler) HandleTextInput(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    userID := int64(update.Message.From.ID)
    text := update.Message.Text

    h.StateMu.RLock()
    userState := h.States[userID]
    h.StateMu.RUnlock()

    if userState == nil {
        h.replyRaw(chatID, "Tôi không hiểu. Gõ /help để xem danh sách lệnh.")
        return
    }

    state, ok := userState["state"].(string)
    if !ok {
        state = StateNone
    }

    switch state {
    case StateAwaitingPassword:
        h.handleInputPassword(chatID, userID, text)

    case StateAwaitingExpiresAt:
        h.handleInputExpiresAt(chatID, userID, text)

    case StateAwaitingConfirm:
        h.handleInputConfirm(chatID, userID, text)

    case "awaiting_auth_password":
		shareIDVal := userState["auth_share_id"]
		if shareIDVal == nil {
			h.replyRaw(chatID, "❌ Lỗi phiên làm việc. Vui lòng bấm lại nút trên tin nhắn.")
			return
		}
		shareID := shareIDVal.(int64)

		// Yêu cầu mật khẩu không rỗng
		password := strings.TrimSpace(text)
		if password == "" {
			h.replyRaw(chatID, "⚠️ Vui lòng nhập mật khẩu để tải file:")
			return
		}

		// Thử authorize với password nhập vào
		token, errAuth := h.ShareSvc.AuthorizeShare(userID, shareID, password)
		
		if errAuth != nil {
			// Authorize thất bại = password sai
			h.replyRaw(chatID, "❌ Mật khẩu sai! Vui lòng nhập lại mật khẩu:")
			return
		}
		
		// Authorize thành công
		h.replyRaw(chatID, "🔓 Mật khẩu chính xác! Đang tải file...")
		
		data, filename, err := h.ShareSvc.DownloadShare(userID, shareID, token)
		if err != nil {
			errMsg := err.Error()
			// Phân biệt lỗi
			if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "404") {
				h.replyRaw(chatID, "❌ Link chia sẻ không tồn tại hoặc đã bị xóa.")
				h.StateMu.Lock()
				delete(h.States, userID)
				h.StateMu.Unlock()
			} else if strings.Contains(errMsg, "revoked") || strings.Contains(errMsg, "expired") {
				h.replyRaw(chatID, "⏰ Link chia sẻ này đã hết hạn hoặc bị thu hồi.")
				h.StateMu.Lock()
				delete(h.States, userID)
				h.StateMu.Unlock()
			} else if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || 
			   strings.Contains(errMsg, "unauthorized") {
				// Token invalid hoặc hết hạn
				h.replyRaw(chatID, "❌ Token hết hạn. Vui lòng nhập mật khẩu lại:")
				// Không xóa state để user nhập lại tiếp
			} else {
				h.replyRaw(chatID, "❌ Lỗi tải file: "+errMsg)
				h.StateMu.Lock()
				delete(h.States, userID)
				h.StateMu.Unlock()
			}
			return
		}
		
		// Tải file thành công
		doc := tgbotapi.FileBytes{Name: filename, Bytes: data}
		h.TG.Send(tgbotapi.NewDocument(chatID, doc))
		
		h.StateMu.Lock()
		delete(h.States, userID)
		h.StateMu.Unlock()


    default:
        h.replyRaw(chatID, "Tôi không hiểu. Gõ /help để xem danh sách lệnh.")
    }
}