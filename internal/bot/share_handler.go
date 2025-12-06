package bot

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// share handler uses ShareService (already in BotHandler)
// Additional share-related helpers can be placed here

func (h *BotHandler) HandleShareCommand(update tgbotapi.Update) {
    chatID := update.Message.Chat.ID
    args := update.Message.CommandArguments()
    if args == "" {
        h.replyRaw(chatID, "Vui lòng cung cấp ShareID hoặc FileID để tạo share. Ví dụ: /share 123")
        return
    }

    // If argument is numeric, assume it's fileID and create a quick share without password
    id, err := strconv.ParseInt(args, 10, 64)
    if err == nil {
        // create simple share (no password)
        share, err := h.ShareSvc.CreateShare(int64(update.Message.From.ID), id, "")
        if err != nil {
            h.replyRaw(chatID, "Tạo share thất bại: "+err.Error())
            return
        }
        link := fmt.Sprintf("https://%s/s/%s", "example.com", share.Hash)
        h.replyRaw(chatID, "Share tạo thành công: "+link)
        return
    }

    h.replyRaw(chatID, "Tham số không hợp lệ. Vui lòng gửi ID dạng số.")
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
