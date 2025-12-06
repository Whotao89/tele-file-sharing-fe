package main

import (
	"fe-file-sharing/internal/api"
	"fe-file-sharing/internal/bot"
	"fe-file-sharing/internal/config"
	"fe-file-sharing/internal/service"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Load config
	config.Load("")

	fmt.Println("Tải config thành công")

	// Create API client (NO MORE ForBot)
	client := api.NewClient(
		config.C.BackendAPIBase,
		0,
		"telegram",
		nil,
	)

	fmt.Println("Tạo client mới thành công")

	// Create Telegram API instance
	tg, err := tgbotapi.NewBotAPI(config.C.TelegramBotToken)
	if err != nil {
		log.Fatalf("Tạo bot Telegram thất bại: %v", err)
	}
	fmt.Println("Tạo bot Telegram thành công")

	// Create services (TV2 implementations)
	uploadSvc := service.NewUploadService(client)
	shareSvc := service.NewShareService(client)
	fileSvc := service.NewFileService(client, config.C.TempDir)
	userSvc := service.NewUserService(client)

	// Create handler and router
	handler := bot.NewBotHandler(tg, uploadSvc, shareSvc, fileSvc, userSvc)
	router := bot.NewRouter(tg, handler)

	fmt.Println("Bắt đầu router Telegram...")
	router.Start()
	fmt.Println("Bot đã bắt đầu chạy")
}
