package bot

import (
	// "fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// share handler uses ShareService (already in BotHandler)
// Additional share-related helpers can be placed here

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
    h.replyRaw(chatID, "Bạn có muốn đặt mật khẩu không?\n👉 Nhập mật khẩu mong muốn hoặc gõ `skip` để bỏ qua:")
}

// AuthorizeShareFlow: user provides password and we call AuthorizeShare
func (h *BotHandler) AuthorizeShareFlow(chatID int64, telegramID int64, shareID int64, password string) {
    token, err := h.ShareSvc.AuthorizeShare(telegramID, shareID, password)
    if err != nil {
        h.replyRaw(chatID, "Xác thực thất bại: "+err.Error())
        return
    }
    h.replyRaw(chatID, "Xác thực thành công. Token: "+token)
}

// DownloadShareFlow: download and send file
func (h *BotHandler) DownloadShareFlow(chatID int64, telegramID int64, shareID int64, token string) {
    data, filename, err := h.ShareSvc.DownloadShare(telegramID, shareID, token)
    if err != nil {
        h.replyRaw(chatID, "Tải file thất bại: "+err.Error())
        return
    }

    // send file as document
    doc := tgbotapi.FileBytes{Name: filename, Bytes: data}
    msg := tgbotapi.NewDocument(chatID, doc)
    h.TG.Send(msg)
}
