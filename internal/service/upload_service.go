package service

import (
	"context"
	"fmt"
	"fe-file-sharing/internal/api"
	"os"
)

type UploadService struct {
	apiClient *api.Client
}

func NewUploadService(client *api.Client) *UploadService {
	return &UploadService{apiClient: client}
}

// StartUpload: Gọi BE lấy URL thật
func (s *UploadService) StartUpload(telegramID int64, filename string, size int64, mimeType string) (int64, string, error) {
	req := api.UploadFileRequest{
		Filename:       filename,
		Size:           size,
		MimeType:       mimeType,
	}

	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	resp, err := s.apiClient.UploadFile(req)
	s.apiClient.TelegramID = prevID

	if err != nil {
		return 0, "", fmt.Errorf("không thể khởi tạo upload: %w", err)
	}

	return resp.FileID, resp.UploadURL, nil
}

// UploadToMinIO: Đẩy file Binary lên URL thật (Dùng helper của TV1)
func (s *UploadService) UploadToMinIO(ctx context.Context, presignedURL string, filePath string) error {
	data := readFileBytes(filePath)
	if data == nil {
		return fmt.Errorf("không đọc được file tại: %s", filePath)
	}

	return api.UploadViaPresignedURL(presignedURL, "", data)
}

// CompleteUpload: Báo cáo thật
func (s *UploadService) CompleteUpload(telegramID int64, fileID int64) error {
	req := api.ReportUploadCompleteRequest{
		Status: "completed",
	}

	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	_, err := s.apiClient.ReportComplete(fileID, req)
	s.apiClient.TelegramID = prevID

	return err
}

func readFileBytes(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}