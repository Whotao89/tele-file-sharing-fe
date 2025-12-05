package main

import (
	"fmt"
	"log"

	"fe-file-sharing/internal/api"
	"fe-file-sharing/internal/bot"
	"fe-file-sharing/internal/config"
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

	// Create bot
	teleBot, err := bot.NewBot(
		config.C.TelegramBotToken,
		client,
	)
	if err != nil {
		log.Fatalf("Tạo bot mới thất bại: %v", err)
	}
	fmt.Println("Tạo bot mới thành công")

	// Start bot
	teleBot.Start()
	fmt.Println("Bot đã bắt đầu chạy")
}
