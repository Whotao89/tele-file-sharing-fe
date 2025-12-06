package service

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

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

// ValidateSize: Kiểm tra dung lượng (Ví dụ giới hạn 50MB)
func (s *FileService) ValidateSize(size int64, limit int64) error {
	if size == 0 {
		return fmt.Errorf("file rỗng")
	}
	if size > limit {
		return fmt.Errorf("kích thước file quá lớn (giới hạn %d MB)", limit/(1024*1024))
	}
	return nil
}

// ValidateMime: Kiểm tra loại file (extension/mime)
func (s *FileService) ValidateMime(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	// Danh sách đen (ví dụ chặn file thực thi)
	blocklist := []string{".exe", ".bat", ".sh", ".msi"}
	for _, blocked := range blocklist {
		if ext == blocked {
			return fmt.Errorf("không hỗ trợ định dạng file thực thi")
		}
	}

	// (Optional) Kiểm tra mime type chuẩn
	mimeType := mime.TypeByExtension(ext)
	_ = mimeType // Có thể log lại nếu cần

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
	prevID := s.apiClient.TelegramID
	s.apiClient.TelegramID = telegramID
	
	files, err := s.apiClient.ListFiles()
	
	s.apiClient.TelegramID = prevID
	
	if err != nil {
		return nil, fmt.Errorf("ListFiles failed: %w", err)
	}
	return files, nil
}
