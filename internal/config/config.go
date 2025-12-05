package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	BackendAPIBase   string
	UseWebhook       bool
	WebhookURL       string
	Port             string
	APITimeout       int
	Debug            bool
	TempDir          string
	FileBaseURL      string
	// APIKey           string
}

var C Config

/*
Load trả về thiết lập từ đường dẫn env cho trước

	VD: env := "env/example.env"
*/
func Load(env string) {
	var err error
	if env != "" {
		err = godotenv.Load(env)
	} else {
		err = godotenv.Load("env/dev.env")
	}
	fmt.Println("Loaded env error =", err)

	C = Config{
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		BackendAPIBase:   getEnv("BE_API_BASE", ""),
		UseWebhook:       getEnvBool("USE_WEBHOOK", false),
		WebhookURL:       getEnv("WEBHOOK_URL", ""),
		Port:             getEnv("PORT", "8000"),
		APITimeout:       getEnvInt("API_TIMEOUT", 10),
		Debug:            getEnvBool("DEBUG", false),
		TempDir:          getEnv("TEMP_DIR", "/tmp/bot"),
		FileBaseURL:      getEnv("FILE_BASE_URL", ""),
		// APIKey:           getEnv("API_KEY", ""),
	}

	validateRequired("TELEGRAM_BOT_TOKEN", C.TelegramBotToken)
	validateRequired("BE_API_BASE", C.BackendAPIBase)
}

// ---------------- HELPERS ----------------
func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	println(fmt.Sprintf("Lấy thành công %s trong env", key))
	return v
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Fatalf("Giá trị boolean không hợp lệ cho %s: %v", key, err)
	}
	println(fmt.Sprintf("Lấy thành công %s trong env", key))
	return b
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("Giá trị integer không hợp lệ cho %s: %v", key, err)
	}
	println(fmt.Sprintf("Lấy thành công %s trong env", key))
	return i
}

func validateRequired(key, value string) {
	if value == "" {
		log.Fatalf("Missing required environment variable: %s", key)
	}
}
