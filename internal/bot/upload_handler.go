package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"fe-file-sharing/internal/config"
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
    if err := h.FileSvc.ValidateSize(int64(doc.FileSize), 50*1024*1024); err != nil {
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
    fileID, uploadURL, err := h.UploadSvc.StartUpload(int64(user.ID), doc.FileName, int64(doc.FileSize), doc.MimeType)
    if err != nil {
        h.replyRaw(chatID, "Không thể khởi tạo upload: "+err.Error())
        return
    }

    // Upload to MinIO (presigned PUT)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()
    if err := h.UploadSvc.UploadToMinIO(ctx, uploadURL, tmpPath); err != nil {
        h.replyRaw(chatID, "Upload thất bại: "+err.Error())
        return
    }

    // Complete upload
    err = h.UploadSvc.CompleteUpload(int64(user.ID), fileID)
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

    created := u.CreatedAt.Format("2006-01-02 15:04:05")

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

        // build view button: prefer URL if FileBaseURL + ObjectKey available
        var btnView tgbotapi.InlineKeyboardButton
        if config.C.FileBaseURL != "" && f.ObjectKey != "" {
            viewURL := strings.TrimRight(config.C.FileBaseURL, "/") + "/" + strings.TrimLeft(f.ObjectKey, "/")
            btnView = tgbotapi.NewInlineKeyboardButtonURL("🔍 View", viewURL)
        } else {
            btnView = tgbotapi.NewInlineKeyboardButtonData("🔍 View", fmt.Sprintf("file:view:%d", f.ID))
        }

        btnShare := tgbotapi.NewInlineKeyboardButtonData("🔗 Share", fmt.Sprintf("cmd:share:%d", f.ID))
        row := tgbotapi.NewInlineKeyboardRow(btnView, btnShare)
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
            h.replyRaw(chatID, "⚠️ Link này đã hết hạn hoặc bị thu hồi.")
            return
        }

        if meta.Revoked {
            h.replyRaw(chatID, "⛔ Link chia sẻ này đã bị thu hồi bởi chủ sở hữu.")
            return
        }

        fileInfo := fmt.Sprintf("🎁 *Bạn nhận được một file chia sẻ!*\n\n📄 Tên: `%s`\n📦 Size: %d bytes", 
            meta.File.Filename, meta.File.Size)
        
        msg := tgbotapi.NewMessage(chatID, fileInfo)
        msg.ParseMode = "Markdown"

        // Kiểm tra xem có cần mật khẩu không (dựa vào Hash hoặc field RequirePassword)
        if meta.Hash != "" { 
             // Hiện nút "Nhập mật khẩu"
            btnAuth := tgbotapi.NewInlineKeyboardButtonData("🔐 Nhập Mật Khẩu", fmt.Sprintf("cmd:auth:%d", shareID))
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnAuth))
        } else {
             // Hiện nút "Tải xuống" luôn
            btnDown := tgbotapi.NewInlineKeyboardButtonData("⬇️ Tải Xuống Ngay", fmt.Sprintf("cmd:download:%d", shareID))
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnDown))
        }
        
        h.TG.Send(msg)
        return
    }       

    welcome := "Xin chào! Tôi là FileShareBot.\nDưới đây là các lệnh bạn có thể dùng:\n" +
        "\n• /me — Xem thông tin tài khoản của bạn.\n" +
		"• /upload — Tải lên một file: gửi file dưới dạng Document trong chat (không gửi ảnh).\n" +
        "• /myfiles — Xem danh sách file bạn đã tải lên.\n" +
        "• /share <file\\_ID> — Tạo link chia sẻ cho một file.\n" +
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
    case data == "cmd:myfiles":
        files, err := h.FileSvc.ListFiles(int64(q.From.ID))
        if err != nil {
            h.replyRaw(chatID, "Lỗi lấy danh sách file: "+err.Error())
            return
        }
        if len(files) == 0 {
            h.replyRaw(chatID, "Bạn chưa có file nào.")
            return
        }
        msg := "Danh sách file:\n"
        for _, f := range files {
            msg += fmt.Sprintf("- %s (%d KB) [id=%d]\n", f.Filename, f.Size/1024, f.ID)
        }
        h.replyRaw(chatID, msg)
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

		h.replyRaw(chatID, "Bạn có muốn đặt mật khẩu không?\n👉 Nhập mật khẩu hoặc gõ `skip` để bỏ qua:")
    // Case: Người nhận bấm nút "Tải xuống"
    case strings.HasPrefix(data, "cmd:download:"):
        parts := strings.Split(data, ":")
        shareID, _ := strconv.ParseInt(parts[2], 10, 64)
        
        h.replyRaw(chatID, "⬇️ Đang lấy file từ server...")
        
        // Gọi Download (Token rỗng vì không cần pass)
        fileData, filename, err := h.ShareSvc.DownloadShare(userID, shareID, "")
        if err != nil {
            h.replyRaw(chatID, "❌ Lỗi tải file: "+err.Error())
            return
        }
        
        doc := tgbotapi.FileBytes{Name: filename, Bytes: fileData}
        h.TG.Send(tgbotapi.NewDocument(chatID, doc))

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
        
        h.replyRaw(chatID, "🔑 Vui lòng nhập mật khẩu để mở khóa file:")
	// ------------------------------
    default:
        h.replyRaw(chatID, "Hành động chưa được hỗ trợ: "+data)
    }
}

// Support view callback when no direct URL is available
// e.g. callback data: file:view:<fileID>
// This will attempt to build a URL from FileBaseURL + object key, or instruct user.
// Note: we handle this inside HandleCallbackQuery's switch above by adding a branch;
// but because the switch has a default already, we add the branch here by patching
// the function above (done inline).

// HandleUploadCommand instructs user to send a file/document
func (h *BotHandler) HandleUploadCommand(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    h.replyRaw(chatID, "Vui lòng gửi file dưới dạng Document (nhấn paperclip -> Document) để bot upload.")
}

// HandleMyShares lists user's shares (stub)
func (h *BotHandler) HandleMyShares(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    // TODO: call ShareService.ListShares when available
    h.replyRaw(chatID, "/myshares đang được triển khai. Dự kiến hiển thị danh sách link bạn tạo.")
}

// HandleRevoke prompts user to provide share id to revoke
func (h *BotHandler) HandleRevoke(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
	userID := int64(update.Message.From.ID)
	args := update.Message.CommandArguments() 

	if args == "" {
		h.replyRaw(chatID, "⚠️ Cách dùng: `/revoke <share_id>`\n(Bạn có thể lấy Share ID khi tạo share hoặc bấm nút Revoke trong danh sách file)")
		return
	}

	shareID, err := strconv.ParseInt(args, 10, 64)
	if err != nil {
		h.replyRaw(chatID, "❌ Share ID phải là một con số. Ví dụ: `/revoke 101`")
		return
	}

	h.replyRaw(chatID, "⏳ Đang thu hồi liên kết...")

	err = h.ShareSvc.RevokeShare(userID, shareID)
	
	if err != nil {
		h.replyRaw(chatID, "❌ Thu hồi thất bại: "+err.Error())
	} else {
		h.replyRaw(chatID, fmt.Sprintf("✅ Đã thu hồi thành công Share ID: `%d`.\nLink này sẽ không còn truy cập được nữa.", shareID))
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

		h.replyRaw(chatID, "⏳ Đang kiểm tra mật khẩu...")

		token, err := h.ShareSvc.AuthorizeShare(userID, shareID, text)
		
		if err != nil {
			h.replyRaw(chatID, "❌ Mật khẩu sai! Vui lòng nhập lại:")
			// Không xóa state để user nhập lại tiếp
			return
		}

		h.replyRaw(chatID, "🔓 Mật khẩu chính xác! Đang tải file...")
		
		data, filename, err := h.ShareSvc.DownloadShare(userID, shareID, token)
		if err != nil {
			h.replyRaw(chatID, "❌ Lỗi tải file: "+err.Error())
		} else {
			// Gửi file
			doc := tgbotapi.FileBytes{Name: filename, Bytes: data}
			h.TG.Send(tgbotapi.NewDocument(chatID, doc))
		}

		h.StateMu.Lock()
		delete(h.States, userID)
		h.StateMu.Unlock()


    default:
        h.replyRaw(chatID, "Tôi không hiểu. Gõ /help để xem danh sách lệnh.")
    }
}

// handleInputPassword: xử lý nhập mật khẩu
func (h *BotHandler) handleInputPassword(chatID int64, userID int64, input string) {
    inputClean := strings.TrimSpace(strings.ToLower(input))

	password := input 

	if inputClean == "skip" || inputClean == "-"  {
		password = "" 
	}

	h.StateMu.Lock()
	if _, exists := h.States[userID]; !exists {
		h.States[userID] = make(map[string]interface{})
	}
	h.States[userID]["password"] = password
	h.States[userID]["state"] = StateAwaitingExpiresAt
	h.StateMu.Unlock()

	msg := ""
	if password == "" {
		msg = "✅ Bạn chọn KHÔNG đặt mật khẩu."
	} else {
		msg = "🔐 Đã lưu mật khẩu."
	}
	
	msg += "\n\n📅 Bước tiếp theo: Nhập ngày hết hạn (định dạng: YYYY-MM-DD) hoặc gõ 'skip' nếu muốn vĩnh viễn:"

	h.replyRaw(chatID, msg)
}

// handleInputExpiresAt: xử lý nhập ngày hết hạn
func (h *BotHandler) handleInputExpiresAt(chatID int64, userID int64, input string) {
    var expiresAt *time.Time

    if input != "-" && input != "none" {
        // Parse ISO date format: 2025-12-12
        t, err := time.Parse("2006-01-02", input)
        if err != nil {
            h.replyRaw(chatID, "❌ Định dạng ngày không hợp lệ. Vui lòng nhập lại (ví dụ: 2025-12-12)")
            return
        }
        expiresAt = &t
    }

    h.StateMu.Lock()
    if _, exists := h.States[userID]; !exists {
        h.States[userID] = make(map[string]interface{})
    }
    h.States[userID]["expiresAt"] = expiresAt
    h.States[userID]["state"] = StateAwaitingConfirm
    h.StateMu.Unlock()

    // Hiển thị thông tin tóm tắt
    h.StateMu.RLock()
    userState := h.States[userID]
    h.StateMu.RUnlock()

    fileID := userState["fileID"].(int64)
    password := userState["password"].(string)

    summary := fmt.Sprintf("📋 Xác nhận thông tin share:\n\n"+
        "📄 File ID: %d\n"+
        "🔐 Mật khẩu: ", fileID)

    if password == "" {
        summary += "Không có"
    } else {
        summary += password
    }

    if expiresAt != nil {
        summary += fmt.Sprintf("\n📅 Hết hạn: %s", expiresAt.Format("2006-01-02"))
    } else {
        summary += "\n📅 Hết hạn: Không có"
    }

    summary += "\n\nGõ 'yes' để xác nhận hoặc 'no' để hủy:"
    h.replyRaw(chatID, summary)
}

// handleInputConfirm: xử lý xác nhận (yes/no)
func (h *BotHandler) handleInputConfirm(chatID int64, userID int64, input string) {
    if input != "yes" && input != "no" {
        h.replyRaw(chatID, "⚠️ Vui lòng gõ 'yes' để xác nhận hoặc 'no' để hủy:")
        return
    }

    if input == "no" {
        h.StateMu.Lock()
        delete(h.States, userID)
        h.StateMu.Unlock()
        h.replyRaw(chatID, "❌ Đã hủy. Gõ /share để bắt đầu lại.")
        return
    }

    // yes: tạo share
    h.StateMu.RLock()
    userState := h.States[userID]
    h.StateMu.RUnlock()

    fileID := userState["fileID"].(int64)
    password := userState["password"].(string)

    // Gọi ShareService.CreateShare
    share, err := h.ShareSvc.CreateShare(userID, fileID, password)
    if err != nil {
        h.replyRaw(chatID, "❌ Tạo share thất bại: "+err.Error())
        h.StateMu.Lock()
        delete(h.States, userID)
        h.StateMu.Unlock()
        return
    }

    // Xóa state
    h.StateMu.Lock()
    delete(h.States, userID)
    h.StateMu.Unlock()

    // Trả lại share link
    shareURL := fmt.Sprintf("https://t.me/%s?start=share_%d", h.TG.Self.UserName, share.ID)
    msg := fmt.Sprintf(
        "✅ Share tạo thành công!\n"+
        "🆔 Share ID: `%d` (Copy số này để Revoke)\n"+
        "🔗 Link: %s", 
        share.ID,
        shareURL,
    )
    
    h.replyRaw(chatID, msg)
}

// helper to read all bytes (unused for now)
func readAll(r io.Reader) ([]byte, error) {
    buf := new(bytes.Buffer)
    _, err := buf.ReadFrom(r)
    if err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}
