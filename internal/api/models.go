package api

import "time"

// -----------------------------
// USER
// -----------------------------
type User struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// -----------------------------
// FILES
// -----------------------------
type File struct {
	ID        int64     `json:"id"`
	ObjectKey string    `json:"object_key"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Mime      string    `json:"mime"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UploadFileRequest struct {
	TelegramFileID string `json:"telegram_file_id"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size"`
	MimeType       string `json:"mime_type,omitempty"`
}

type UploadFileResponse struct {
	FileID    int64  `json:"file_id"`
	Status    string `json:"status"`
	UploadURL string `json:"upload_url"`
}

type ReportUploadCompleteRequest struct {
	Status           string  `json:"status"`
	ReportType       string  `json:"report_type"`
	Message          string  `json:"message,omitempty"`
	ErrorCode        string  `json:"error_error,omitempty"`
	ErrorMessage     string  `json:"error_message,omitempty"`
	FileChecksum     string  `json:"file_checksum,omitempty"`
	FileSizeActual   int64   `json:"file_size_actual,omitempty"`
	UploadDurationMs int64   `json:"upload_duration_ms,omitempty"`
	BandwidthKbps    float64 `json:"bandwidth_kbps,omitempty"`
}

type ReportUploadCompleteResponse struct {
	ReportID   int64     `json:"report_id"`
	FileID     int64     `json:"file_id"`
	Status     string    `json:"status"`
	ReportType string    `json:"report_type"`
	Message    string    `json:"message"`
	ReportedAt time.Time `json:"reported_at"`
}

type UploadReport struct {
	ID               int64     `json:"id"`
	FileID           int64     `json:"file_id"`
	OwnerUserID      int64     `json:"owner_user_id"`
	Status           string    `json:"status"`
	ReportType       string    `json:"report_type"`
	Message          string    `json:"message"`
	ErrorCode        string    `json:"error_code"`
	ErrorMessage     string    `json:"error_message"`
	FileChecksum     string    `json:"file_checksum"`
	FileSizeActual   int64     `json:"file_size_actual"`
	UploadDurationMs int64     `json:"upload_duration_ms"`
	BandwidthKbps    float64   `json:"bandwidth_kbps"`
	ReportedAt       time.Time `json:"reported_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type UploadReportsListResponse []UploadReport

// -----------------------------
// SHARES
// -----------------------------
type CreateShareRequest struct {
	FileID    int64      `json:"file_id"`
	Password  string     `json:"password,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Share struct {
	ID          int64      `json:"id"`
	FileID      int64      `json:"file_id"`
	OwnerUserID int64      `json:"owner_user_id"`
	Hash        string     `json:"hash"`
	RequirePwd  bool       `json:"require_password"`
	Revoked     bool       `json:"revoked"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ShareRevokeResponse struct {
	Message string `json:"message"`
	ShareID int64  `json:"share_id"`
	Status  string `json:"status"`
}

type ShareAuthorizeRequest struct {
	Password string `json:"password"`
}

type ShareAuthorizeResponse struct {
	AccessToken string `json:"access_token"`
}

type FileMetadata struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	ObjectKey string `json:"object_key"`
	Size      int64  `json:"size"`
	Mime      string `json:"mime"`
	Status    string `json:"status"`
}

type UserMinimal struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	TelegramUserID int64  `json:"telegram_user_id"`
}

type ShareMetadataResponse struct {
	ID        int64        `json:"id"`
	Hash      string       `json:"hash"`
	RequirePassword bool `json:"require_password"`
	Revoked   bool         `json:"revoked"`
	ExpiresAt *time.Time   `json:"expires_at"`
	CreatedAt time.Time    `json:"created_at"`
	File      FileMetadata `json:"file"`
	Owner     UserMinimal  `json:"owner"`
}

// -----------------------------
// ERROR
// -----------------------------
type APIError struct {
	Error string `json:"error"`
}
