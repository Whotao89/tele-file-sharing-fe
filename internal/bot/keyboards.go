package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MainMenuKeyboard returns a reply keyboard with main bot commands
func MainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
    row1 := tgbotapi.NewKeyboardButtonRow(
        tgbotapi.NewKeyboardButton("📂 Quản lý File"), 
        tgbotapi.NewKeyboardButton("🔗 Quản lý Share Link"),    
    )
    row2 := tgbotapi.NewKeyboardButtonRow(
        tgbotapi.NewKeyboardButton("ℹ️ Trợ giúp"),    
    )

    kb := tgbotapi.NewReplyKeyboard(row1, row2)
    kb.ResizeKeyboard = true
    kb.OneTimeKeyboard = false
    return kb
}

// PostUploadInline returns buttons shown after a successful upload
func PostUploadInline(fileID int64) tgbotapi.InlineKeyboardMarkup {
    // Only keep Share to avoid clutter after upload
    btnShare := tgbotapi.NewInlineKeyboardButtonData("🔗 /share - Chia sẻ file", fmt.Sprintf("cmd:share:%d", fileID))
    row := tgbotapi.NewInlineKeyboardRow(btnShare)
    return tgbotapi.NewInlineKeyboardMarkup(row)
}
