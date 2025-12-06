package bot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type BotRouter struct {
    TG      *tgbotapi.BotAPI
    Handler *BotHandler
}

func NewRouter(tg *tgbotapi.BotAPI, h *BotHandler) *BotRouter {
    return &BotRouter{TG: tg, Handler: h}
}

// Start listens for Telegram updates and dispatches to handlers
func (r *BotRouter) Start() {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 30
    updates := r.TG.GetUpdatesChan(u)

    for update := range updates {
        // handle callback queries first
        if update.CallbackQuery != nil {
            r.Handler.HandleCallbackQuery(update.CallbackQuery)
            continue
        }

        if update.Message == nil {
            continue
        }

        // if message contains a document -> upload
        if update.Message.Document != nil {
            go r.Handler.HandleUpload(update)
            continue
        }

        // simple routing by command
        cmd := update.Message.Command()
        if cmd != "" {
            switch cmd {
            case "start":
                r.Handler.HandleStart(update)
            case "me":
                r.Handler.HandleMe(update)
            case "upload":
                r.Handler.HandleUploadCommand(update)
            case "myfiles":
                r.Handler.HandleFiles(update)
            case "share":
                r.Handler.HandleShareCommand(update)
            // case "myshares":
            //     r.Handler.HandleMyShares(update)
            case "revoke":
                r.Handler.HandleRevoke(update)
            }
            continue
        }

        if update.Message.Text != "" {
            text := update.Message.Text

				if text == "📂 Quản lý File" {
					go r.Handler.HandleFiles(update)
					continue
				}
				
				if text == "ℹ️ Trợ giúp" {
					go r.Handler.HandleStart(update)
					continue
				}

				// Nếu không phải nút bấm thì mới coi là nhập liệu (Wizard)
				go r.Handler.HandleTextInput(update)
        }
    }
}
