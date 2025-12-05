package bot

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"fe-file-sharing/internal/api"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	TG     *tgbotapi.BotAPI
	Client *api.Client
}

func NewBot(token string, client *api.Client) (*Bot, error) {
	tg, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{TG: tg, Client: client}, nil
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.TG.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "me":
			b.commandMe(update)
		case "files":
			b.commandFiles(update)
		}

		if update.Message.Document != nil {
			b.handleUpload(update)
		}
	}
}

func (b *Bot) commandMe(upd tgbotapi.Update) {
	user, err := b.Client.GetMe()
	if err != nil {
		b.reply(upd, "Lỗi kiếm người dùng: "+err.Error())
		return
	}

	msg := fmt.Sprintf(
		"ID: %d\nTelegram: %d\nUsername: %s",
		user.ID, user.TelegramID, user.Username,
	)

	b.reply(upd, msg)
}

func (b *Bot) commandFiles(upd tgbotapi.Update) {
	files, err := b.Client.ListFiles()
	if err != nil {
		b.reply(upd, "Lỗi xuất danh sách: "+err.Error())
		return
	}

	if len(files) == 0 {
		b.reply(upd, "Không có file nào.")
		return
	}

	msg := "*Danh sách file:*\n"
	for _, f := range files {
		msg += fmt.Sprintf("- %s (%d KB)\n", f.Filename, f.Size/1024)
	}

	b.reply(upd, msg)
}

func (b *Bot) handleUpload(upd tgbotapi.Update) {
	doc := upd.Message.Document

	file, err := b.TG.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		b.reply(upd, "Không tải được file telegram")
		return
	}

	url := file.Link(b.TG.Token)
	data, err := downloadURL(url)
	if err != nil {
		b.reply(upd, "Lỗi tải xuống: "+err.Error())
		return
	}

	tmp := filepath.Join(os.TempDir(), doc.FileName)
	os.WriteFile(tmp, data, 0644)

	// gửi đủ thông tin
	req := api.UploadFileRequest{
		TelegramFileID: doc.FileID,
		Filename:       doc.FileName,
		Size:           int64(doc.FileSize),
		MimeType:       doc.MimeType,
	}

	resp, err := b.Client.UploadFile(req)
	if err != nil {
		b.reply(upd, "Lỗi tải lên: "+err.Error())
		return
	}

	b.reply(upd, fmt.Sprintf("Tải lên thành công! File ID: %d", resp.FileID))
}

func downloadURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (b *Bot) reply(upd tgbotapi.Update, text string) {
	msg := tgbotapi.NewMessage(upd.Message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	b.TG.Send(msg)
}
