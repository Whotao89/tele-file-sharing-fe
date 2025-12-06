package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ConfirmKeyboard() tgbotapi.InlineKeyboardMarkup {
    btnYes := tgbotapi.NewInlineKeyboardButtonData("Yes", "confirm_yes")
    btnNo := tgbotapi.NewInlineKeyboardButtonData("No", "confirm_no")
    row := tgbotapi.NewInlineKeyboardRow(btnYes, btnNo)
    return tgbotapi.NewInlineKeyboardMarkup(row)
}

// MainMenuKeyboard returns a reply keyboard with main bot commands
func MainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
    row1 := tgbotapi.NewKeyboardButtonRow(
        tgbotapi.NewKeyboardButton("📂 Quản lý File"), 
        tgbotapi.NewKeyboardButton("ℹ️ Trợ giúp"),    
    )

    kb := tgbotapi.NewReplyKeyboard(row1)
    kb.ResizeKeyboard = true
    kb.OneTimeKeyboard = false
    return kb
}

// PostUploadInline returns two buttons shown after a successful upload
func PostUploadInline(fileID int64) tgbotapi.InlineKeyboardMarkup {
    btnMyFiles := tgbotapi.NewInlineKeyboardButtonData("📁 /myfiles - Xem danh sách", "cmd:myfiles")
    btnShare := tgbotapi.NewInlineKeyboardButtonData("🔗 /share - Chia sẻ file", fmt.Sprintf("cmd:share:%d", fileID))
    row1 := tgbotapi.NewInlineKeyboardRow(btnMyFiles)
    row2 := tgbotapi.NewInlineKeyboardRow(btnShare)
    return tgbotapi.NewInlineKeyboardMarkup(row1, row2)
}
