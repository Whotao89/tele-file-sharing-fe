package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fe-file-sharing/internal/api"
)

type FileService struct {
	TempDir   string
	apiClient *api.Client
}

func NewFileService(client *api.Client, tempDir string) *FileService {
	// Đảm bảo thư mục tạm tồn tại
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		_ = os.MkdirAll(tempDir, 0755)
	}
	return &FileService{TempDir: tempDir, apiClient: client}
}

// ValidateSize: Kiểm tra dung lượng (Ví dụ giới hạn 100MB)
func (s *FileService) ValidateSize(size int64, limit int64) error {
	if size == 0 {
		return fmt.Errorf("file rỗng")
	}
	if size > limit {
		return fmt.Errorf("kích thước file quá lớn (giới hạn %d MB)", limit/(1024*1024))
	}
	return nil
}

// SaveTempFile: Lưu stream từ Telegram xuống ổ cứng để chuẩn bị upload
func (s *FileService) SaveTempFile(r io.Reader, filename string) (string, error) {
	// Tạo đường dẫn file an toàn
	safeName := filepath.Base(filename)
	path := filepath.Join(s.TempDir, safeName)

	out, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("lỗi tạo file tạm: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, r)
	if err != nil {
		return "", fmt.Errorf("lỗi ghi dữ liệu: %w", err)
	}

	return path, nil
}

// CleanUp: Xóa file tạm sau khi upload xong
func (s *FileService) CleanUp(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

// ListFiles: gọi backend trả về danh sách file của user
func (s *FileService) ListFiles(telegramID int64) ([]api.File, error) {
	prev := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	files, err := s.apiClient.ListFiles()
	s.apiClient.TelegramID = prev
	if err != nil {
		return nil, fmt.Errorf("ListFiles failed: %w", err)
	}
	return files, nil
}

// GetUploadReports: Lấy danh sách upload reports
func (s *FileService) GetUploadReports(telegramID int64, limit, offset int) ([]api.UploadReport, error) {
	prev := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	reports, err := s.apiClient.GetUploadReports(limit, offset)
	s.apiClient.TelegramID = prev
	if err != nil {
		return nil, fmt.Errorf("GetUploadReports failed: %w", err)
	}
	return reports, nil
}

// GetFileReport: Lấy upload report của 1 file cụ thể
func (s *FileService) GetFileReport(telegramID int64, fileID int64) (*api.UploadReport, error) {
	prev := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	report, err := s.apiClient.GetFileReport(fileID)
	s.apiClient.TelegramID = prev
	if err != nil {
		return nil, fmt.Errorf("GetFileReport failed: %w", err)
	}
	return report, nil
}
