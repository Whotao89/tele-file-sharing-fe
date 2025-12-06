package service

import (
	"fe-file-sharing/internal/api"
	"fmt"
)

type ShareService struct {
	apiClient *api.Client
}

func NewShareService(client *api.Client) *ShareService {
	return &ShareService{apiClient: client}
}

// ShareServiceIface defines the subset of share operations the bot needs.
// Both real ShareService and MockShareService implement this interface.
type ShareServiceIface interface {
	CreateShare(telegramID int64, fileID int64, password string) (*api.Share, error)
	AuthorizeShare(telegramID int64, shareID int64, password string) (string, error)
	DownloadShare(telegramID int64, shareID int64, accessToken string) ([]byte, string, error)
}

// CreateShare: Tạo link chia sẻ
// Truyền telegramID để header được set trên api client
func (s *ShareService) CreateShare(telegramID int64, fileID int64, password string) (*api.Share, error) {
	req := api.CreateShareRequest{
		FileID:   fileID,
		Password: password,
	}

	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	share, err := s.apiClient.CreateShare(req)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return nil, fmt.Errorf("tạo share thất bại: %w", err)
	}
	return share, nil
}

// GetShareMetadata: Lấy thông tin file share
func (s *ShareService) GetShareMetadata(telegramID int64, shareID int64) (*api.ShareMetadataResponse, error) {
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	meta, err := s.apiClient.GetShareMetadata(shareID)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return nil, fmt.Errorf("lỗi lấy thông tin share: %w", err)
	}
	return meta, nil
}

// AuthorizeShare: Nhập mật khẩu mở khóa share (trả access token)
func (s *ShareService) AuthorizeShare(telegramID int64, shareID int64, password string) (string, error) {
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	resp, err := s.apiClient.AuthorizeShare(shareID, password)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return "", fmt.Errorf("xác thực thất bại: %w", err)
	}
	return resp.AccessToken, nil
}

// DownloadShare: Tải nội dung file
func (s *ShareService) DownloadShare(telegramID int64, shareID int64, accessToken string) ([]byte, string, error) {
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	headers := map[string]string{}
	if accessToken != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
	}
	data, filename, err := s.apiClient.DownloadShare(shareID, headers)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return nil, "", fmt.Errorf("tải file thất bại: %w", err)
	}
	return data, filename, nil
}