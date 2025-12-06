package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Simple helpers for extracting Telegram identity from update
func GetTelegramIdentity(update tgbotapi.Update) (int64, string) {
    if update.Message == nil || update.Message.From == nil {
        return 0, ""
    }
    return int64(update.Message.From.ID), update.Message.From.UserName
}
