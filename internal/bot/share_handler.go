package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) HandleShareCommand(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    userID := int64(update.Message.From.ID)
    args := update.Message.CommandArguments()

    // 1. Kiểm tra tham số
    if args == "" {
        h.replyRaw(chatID, "⚠️ Vui lòng nhập ID file. Ví dụ: `/share 123`")
        return
    }

    fileID, err := strconv.ParseInt(args, 10, 64)
    if err != nil {
        h.replyRaw(chatID, "❌ ID file phải là số. Ví dụ: `/share 123`")
        return
    }

    // 2. Kích hoạt Wizard (Giống hệt lúc bấm nút)
    h.StateMu.Lock()
    if h.States[userID] == nil {
        h.States[userID] = make(map[string]interface{})
    }
    // Lưu ID file vào bộ nhớ
    h.States[userID]["fileID"] = fileID
    // Chuyển trạng thái sang bước 1: Đợi nhập mật khẩu
    h.States[userID]["state"] = StateAwaitingPassword
    h.StateMu.Unlock()

    // 3. Hỏi người dùng
    h.replyRaw(chatID, "🔐 Vui lòng nhập mật khẩu để tạo chia sẻ:")
}


// HandleMyShares lists user's shares
func (h *BotHandler) HandleMyShares(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    userID := int64(update.Message.From.ID)

    // Gọi ShareService để lấy danh sách shares
    shares, err := h.ShareSvc.ListShares(userID, 50, 0)
    if err != nil {
        h.replyRaw(chatID, "Lỗi lấy danh sách share: "+err.Error())
        return
    }

    if len(shares) == 0 {
        h.replyRaw(chatID, "Bạn chưa tạo chia sẻ nào.")
        return
    }

    // Hiển thị danh sách shares (split thành nhiều message nếu quá dài)
    text := "📋 Danh sách các chia sẻ của bạn:\n\n"
    now := time.Now()
    messageCount := 0
    
    for i, s := range shares {
        // Determine status with expiry awareness
        status := "✅ Hoạt động"
        if s.Revoked {
            status = "❌ Đã thu hồi"
        } else if s.ExpiresAt != nil && s.ExpiresAt.Before(now) {
            status = "⏰ Đã hết hạn"
        }
        
        
        expiryText := "Không có"
        if s.ExpiresAt != nil {
            expiryText = s.ExpiresAt.In(time.Local).Format("02/01/2006 15:04:05")
        }
        
        // Build share URL: dùng Telegram deep link /start?share_={ID}
        shareURL := fmt.Sprintf("https://t.me/%s?start=share_%d", h.BotUsername, s.ID)
        
        entry := fmt.Sprintf("%d. Share ID: %d | File ID: %d\n   Link: %s\n   %s\n   Hết hạn: %s\n   Tạo: %s\n\n", 
            i+1, s.ID, s.FileID, shareURL, status, expiryText, 
            s.CreatedAt.In(time.Local).Format("02/01/2006 15:04:05"))
        
        // Nếu message sắp vượt 4000 chars → gửi và reset
        if len(text) + len(entry) > 3900 {
            h.replyRaw(chatID, text)
            messageCount++
            text = "📋 Danh sách chia sẻ (tiếp theo):\n\n"
        }
        
        text += entry
    }
    
    if text != "📋 Danh sách chia sẻ (tiếp theo):\n\n" {
        h.replyRaw(chatID, text)
    }
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


// handleInputPassword: xử lý nhập mật khẩu
func (h *BotHandler) handleInputPassword(chatID int64, userID int64, input string) {
    password := strings.TrimSpace(input)
    if password == "" {
        h.replyRaw(chatID, "⚠️ Mật khẩu không được để trống. Vui lòng nhập lại:")
        return
    }

    h.StateMu.Lock()
    if _, exists := h.States[userID]; !exists {
        h.States[userID] = make(map[string]interface{})
    }
    h.States[userID]["password"] = password
    h.States[userID]["state"] = StateAwaitingExpiresAt
    h.StateMu.Unlock()

    msg := "🔐 Đã lưu mật khẩu."
    msg += "\n\n📅 Bước tiếp theo: Nhập ngày hết hạn (định dạng: DD/MM/YYYY) hoặc gõ 'skip' nếu muốn vĩnh viễn:"

    h.replyRaw(chatID, msg)
}

// handleInputExpiresAt: xử lý nhập ngày hết hạn
func (h *BotHandler) handleInputExpiresAt(chatID int64, userID int64, input string) {
    var expiresAt *time.Time
    
    inputClean := strings.TrimSpace(strings.ToLower(input))

    if inputClean != "skip" {
        // Parse DD/MM/YYYY date format: 10/12/2025 in local timezone
        t, err := time.ParseInLocation("02/01/2006", input, time.Local)
        if err != nil {
            h.replyRaw(chatID, "❌ Định dạng ngày không hợp lệ. Vui lòng nhập lại (ví dụ: 10/12/2025) hoặc gõ 'skip' để vĩnh viễn:")
            return
        }
        // Set to 23:59:59 of the selected day (in local time)
        t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
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
        "🔐 Mật khẩu: %s", fileID, password)

    if expiresAt != nil {
        summary += fmt.Sprintf("\n📅 Hết hạn: %s", expiresAt.Format("02/01/2006 15:04:05"))
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
    expiresAt, _ := userState["expiresAt"].(*time.Time)

    // Gọi ShareService.CreateShare
    share, err := h.ShareSvc.CreateShare(userID, fileID, password, expiresAt)
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
