package service

import (
	"fe-file-sharing/internal/api"
	"fmt"
	"time"
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
	CreateShare(telegramID int64, fileID int64, password string, expiresAt *time.Time) (*api.Share, error)
	AuthorizeShare(telegramID int64, shareID int64, password string) (string, error)
	DownloadShare(telegramID int64, shareID int64, accessToken string) ([]byte, string, error)
}

// CreateShare: Tạo link chia sẻ
// Truyền telegramID để header được set trên api client
func (s *ShareService) CreateShare(telegramID int64, fileID int64, password string, expiresAt *time.Time) (*api.Share, error) {
	req := api.CreateShareRequest{
		FileID:   fileID,
		Password: password,
		ExpiresAt: expiresAt,
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

func (s *ShareService) RevokeShare(telegramID int64, shareID int64) error {
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	
	// Gọi client TV1
	_, err := s.apiClient.RevokeShare(shareID)
	
	s.apiClient.TelegramID = prevID
	return err
}

// ListShares: Lấy danh sách các chia sẻ của user
func (s *ShareService) ListShares(telegramID int64, limit int, offset int) ([]api.Share, error) {
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	shares, err := s.apiClient.ListShares(limit, offset)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return nil, fmt.Errorf("lỗi lấy danh sách share: %w", err)
	}
	return shares, nil
}