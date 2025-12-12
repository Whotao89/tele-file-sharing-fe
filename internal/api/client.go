package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	HTTP       *http.Client
	TelegramID int64
	Username   string
}

func NewClient(baseURL string, telegramID int64, username string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:    baseURL,
		HTTP:       httpClient,
		TelegramID: telegramID,
		Username:   username,
	}
}

// Create a client for bot usage
func NewClientForBot(baseURL string, httpClient *http.Client) *Client {
	return NewClient(baseURL, 0, "bot", httpClient)
}

// Generic request executor
func (c *Client) request(method, path string, body any, out any) error {
	var reader io.Reader

	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewBuffer(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-User-Id", fmt.Sprintf("%d", c.TelegramID))
	req.Header.Set("X-Telegram-Username", c.Username)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		var apiErr APIError
		if json.Unmarshal(data, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("API Error: %s", apiErr.Error)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

//////////////////////////////////////
//              FILES
//////////////////////////////////////

// POST /v1/files  (init upload)
func (c *Client) UploadFile(req UploadFileRequest) (*UploadFileResponse, error) {
	var out UploadFileResponse
	err := c.request("POST", "/api/v1/files", req, &out)
	return &out, err
}

// POST /v1/files/{id}/report-complete
func (c *Client) ReportComplete(id int64, req ReportUploadCompleteRequest) (*ReportUploadCompleteResponse, error) {
	var out ReportUploadCompleteResponse
	err := c.request("POST", fmt.Sprintf("/api/v1/files/%d/report-complete", id), req, &out)
	return &out, err
}

// GET /v1/files
func (c *Client) ListFiles() ([]File, error) {
	var out []File
	err := c.request("GET", "/api/v1/files", nil, &out)
	return out, err
}

// GET /v1/files/{id}/report
func (c *Client) GetFileReport(id int64) (*UploadReport, error) {
	var out UploadReport
	err := c.request("GET", fmt.Sprintf("/api/v1/files/%d/report", id), nil, &out)
	return &out, err
}

// GET /v1/upload-reports
func (c *Client) GetUploadReports(limit, offset int) ([]UploadReport, error) {
	var out []UploadReport
	err := c.request("GET", fmt.Sprintf("/api/v1/upload-reports?limit=%d&offset=%d", limit, offset), nil, &out)
	return out, err
}

//	SHARES
//
// POST /v1/shares
func (c *Client) CreateShare(req CreateShareRequest) (*Share, error) {
	var out Share
	err := c.request("POST", "/api/v1/shares", req, &out)
	return &out, err
}

// GET /v1/shares
func (c *Client) ListShares(limit, offset int) ([]Share, error) {
	var out []Share
	err := c.request("GET", fmt.Sprintf("/api/v1/shares?limit=%d&offset=%d", limit, offset), nil, &out)
	if err == nil && len(out) > 0 {
		return out, nil
	}

	// Backend trả wrapper object {shares: [...]} thay vì array trực tiếp
	type SharesWrapper struct {
		Data   []Share `json:"data"`
		Shares []Share `json:"shares"`
	}
	var wrapper SharesWrapper
	err2 := c.request("GET", fmt.Sprintf("/api/v1/shares?limit=%d&offset=%d", limit, offset), nil, &wrapper)
	if err2 == nil {
		if len(wrapper.Shares) > 0 {
			return wrapper.Shares, nil
		}
		if len(wrapper.Data) > 0 {
			return wrapper.Data, nil
		}
	}

	// Nếu cả 2 cách đều fail, return error gốc
	return out, err
}

// GET /v1/shares/{id}
func (c *Client) GetShareMetadata(id int64) (*ShareMetadataResponse, error) {
	var out ShareMetadataResponse
	err := c.request("GET", fmt.Sprintf("/api/v1/shares/%d", id), nil, &out)
	return &out, err
}

// POST /v1/shares/{id}/authorize
func (c *Client) AuthorizeShare(id int64, password string) (*ShareAuthorizeResponse, error) {
	req := ShareAuthorizeRequest{Password: password}
	var out ShareAuthorizeResponse
	err := c.request("POST", fmt.Sprintf("/api/v1/shares/%d/authorize", id), req, &out)
	return &out, err
}

// GET /api/v1/files/{id}/download
// Download file cho file owner
func (c *Client) DownloadFile(fileID int64) ([]byte, string, error) {
	url := c.BaseURL + fmt.Sprintf("/api/v1/files/%d/download", fileID)
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Set("X-Telegram-User-Id", fmt.Sprintf("%d", c.TelegramID))
	req.Header.Set("X-Telegram-Username", c.Username)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download failed: %s", string(b))
	}

	// Parse presigned URL from response
	var presignedResp struct {
		URL       string `json:"url"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&presignedResp); err != nil {
		return nil, "", fmt.Errorf("invalid response: %w", err)
	}

	// Download from presigned URL
	presignedReq, _ := http.NewRequest("GET", presignedResp.URL, nil)
	presignedResp2, err := c.HTTP.Do(presignedReq)
	if err != nil {
		return nil, "", fmt.Errorf("presigned download failed: %w", err)
	}
	defer presignedResp2.Body.Close()

	if presignedResp2.StatusCode >= 400 {
		b, _ := io.ReadAll(presignedResp2.Body)
		return nil, "", fmt.Errorf("presigned download failed: %s", string(b))
	}

	// Read file binary
	data, err := io.ReadAll(presignedResp2.Body)
	if err != nil {
		return nil, "", err
	}

	// Get filename from list files
	files, err := c.ListFiles()
	if err != nil {
		return data, "file", nil // Fallback
	}

	for _, f := range files {
		if f.ID == fileID {
			return data, f.Filename, nil
		}
	}

	return data, "file", nil
}

// GET /v1/shares/{id}/download
// Bước 1: Lấy presigned URL từ BE
func (c *Client) DownloadShare(id int64, headers map[string]string) ([]byte, string, error) {
	// Bước 0: Lấy metadata share để có filename
	var metadata ShareMetadataResponse
	if err := c.request("GET", fmt.Sprintf("/api/v1/shares/%d", id), nil, &metadata); err != nil {
		return nil, "", fmt.Errorf("failed to get share metadata: %w", err)
	}
	
	filename := metadata.File.Filename
	
	url := c.BaseURL + fmt.Sprintf("/api/v1/shares/%d/download", id)
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Set("X-Telegram-User-Id", fmt.Sprintf("%d", c.TelegramID))
	req.Header.Set("X-Telegram-Username", c.Username)

	for k, v := range headers {
		// Convert Authorization header to X-Share-Token for BE compatibility
		if k == "Authorization" && strings.HasPrefix(v, "Bearer ") {
			req.Header.Set("X-Share-Token", strings.TrimPrefix(v, "Bearer "))
		} else {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download failed: %s", string(b))
	}

	// Parse presigned URL từ response
	var presignedResp struct {
		URL       string `json:"url"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&presignedResp); err != nil {
		return nil, "", fmt.Errorf("invalid response: %w", err)
	}

	// Bước 2: Download từ presigned URL
	presignedReq, _ := http.NewRequest("GET", presignedResp.URL, nil)
	presignedResp2, err := c.HTTP.Do(presignedReq)
	if err != nil {
		return nil, "", fmt.Errorf("presigned download failed: %w", err)
	}
	defer presignedResp2.Body.Close()

	if presignedResp2.StatusCode >= 400 {
		b, _ := io.ReadAll(presignedResp2.Body)
		return nil, "", fmt.Errorf("presigned download failed: %s", string(b))
	}

	// Đọc file binary
	data, err := io.ReadAll(presignedResp2.Body)
	if err != nil {
		return nil, "", err
	}

	return data, filename, nil
}

// POST /v1/shares/{id}/revoke
func (c *Client) RevokeShare(id int64) (*ShareRevokeResponse, error) {
	var out ShareRevokeResponse
	err := c.request("POST", fmt.Sprintf("/api/v1/shares/%d/revoke", id), nil, &out)
	return &out, err
}

//	USER
//
// GET /api/me
func (c *Client) GetMe() (*User, error) {
	var out User
	err := c.request("GET", "/api/me", nil, &out)
	return &out, err
}

//	PRESIGNED URL UPLOAD
//
// Upload file directly via Presigned URL
func UploadViaPresignedURL(uploadURL, mime string, data []byte) error {
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", mime)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", string(b))
	}

	return nil
}
