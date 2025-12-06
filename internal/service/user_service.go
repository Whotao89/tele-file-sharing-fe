package service

import (
	"fmt"

	"fe-file-sharing/internal/api"
)

type UserService struct {
    apiClient *api.Client
}

func NewUserService(client *api.Client) *UserService {
    return &UserService{apiClient: client}
}

// GetMe calls backend to retrieve the current user mapped from Telegram headers
func (s *UserService) GetMe(telegramID int64, username string) (*api.User, error) {
	client := *s.apiClient 
	
	client.TelegramID = telegramID
	client.Username = username

	user, err := client.GetMe()
	if err != nil {
		return nil, fmt.Errorf("GetMe failed: %w", err)
	}
	return user, nil
}
