package google

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"

	"mysql-backup/internal/config"
)

type Client struct {
	config *config.Config
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type DriveFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		config: cfg,
	}
}

func (c *Client) GetAuthURL() string {
	baseURL := "https://accounts.google.com/o/oauth2/auth"
	params := url.Values{}
	params.Add("client_id", c.config.Google.ClientID)
	params.Add("redirect_uri", "http://localhost:8030/api/auth/google/callback")
	params.Add("scope", "https://www.googleapis.com/auth/drive.file https://www.googleapis.com/auth/spreadsheets")
	params.Add("response_type", "code")
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")

	return baseURL + "?" + params.Encode()
}

func (c *Client) ExchangeCode(code string) error {
	fmt.Printf("Exchanging Google OAuth code: %s\n", code[:10]+"...")

	tokenURL := "https://oauth2.googleapis.com/token"

	data := url.Values{}
	data.Set("client_id", c.config.Google.ClientID)
	data.Set("client_secret", c.config.Google.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", "http://localhost:8030/api/auth/google/callback")

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Token exchange failed. Status: %d, Body: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	// Save tokens to config
	c.config.Google.AccessToken = tokenResp.AccessToken
	c.config.Google.RefreshToken = tokenResp.RefreshToken
	c.config.Google.TokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	fmt.Println("Google OAuth tokens saved successfully")
	return c.config.Save()
}

func (c *Client) RefreshToken() error {
	if c.config.Google.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	fmt.Println("Refreshing Google access token...")

	tokenURL := "https://oauth2.googleapis.com/token"

	data := url.Values{}
	data.Set("client_id", c.config.Google.ClientID)
	data.Set("client_secret", c.config.Google.ClientSecret)
	data.Set("refresh_token", c.config.Google.RefreshToken)
	data.Set("grant_type", "refresh_token")

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Token refresh failed. Status: %d, Body: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	// Update access token
	c.config.Google.AccessToken = tokenResp.AccessToken
	c.config.Google.TokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)

	fmt.Println("Google access token refreshed successfully")
	return c.config.Save()
}

func (c *Client) UploadFile(filePath, fileName string) (string, error) {
	// Check if token needs refresh
	if err := c.ensureValidToken(); err != nil {
		return "", fmt.Errorf("failed to ensure valid token: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	fmt.Printf("Uploading file: %s (%.2f MB)\n", fileName, float64(fileInfo.Size())/(1024*1024))

	// Create metadata
	metadata := map[string]interface{}{
		"name": fileName,
	}

	if c.config.Google.DriveFolder != "" {
		metadata["parents"] = []string{c.config.Google.DriveFolder}
	}

	metadataJSON, _ := json.Marshal(metadata)

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add metadata part
	metadataPart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"application/json; charset=UTF-8"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create metadata part: %w", err)
	}
	metadataPart.Write(metadataJSON)

	// Add file content part
	filePart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"application/gzip"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create file part: %w", err)
	}

	if _, err := io.Copy(filePart, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	writer.Close()

	// Upload to Google Drive
	req, err := http.NewRequest("POST", "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart", &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Google.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Minute} // Timeout maior para arquivos grandes
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Upload failed. Status: %d, Body: %s\n", resp.StatusCode, string(body))
		return "", fmt.Errorf("upload failed with status: %d - %s", resp.StatusCode, string(body))
	}

	var driveFile DriveFile
	if err := json.NewDecoder(resp.Body).Decode(&driveFile); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("File uploaded successfully to Google Drive: %s\n", driveFile.ID)
	return driveFile.ID, nil
}

func (c *Client) LogToSheets(log config.BackupLog) error {
	if c.config.Google.SheetID == "" {
		return nil // Sheets logging not configured
	}

	// Check if token needs refresh
	if err := c.ensureValidToken(); err != nil {
		return err
	}

	// Formato solicitado: STATUS | DATA/HORA | NOME_ARQUIVO | LOG
	status := "ERRO"
	logMessage := log.Error
	if log.Success {
		status = "SUCESSO"
		logMessage = fmt.Sprintf("Backup da database %s criado com sucesso", log.TableName)
	}

	// Prepare row data no formato solicitado
	values := [][]interface{}{{
		status, // STATUS: SUCESSO ou ERRO
		log.Timestamp.Format("02/01/2006H15:04:05"), // DATA/HORA no formato brasileiro
		log.FileName, // NOME_ARQUIVO
		logMessage,   // LOG
	}}

	requestBody := map[string]interface{}{
		"values": values,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/A:D:append?valueInputOption=RAW", c.config.Google.SheetID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to log to sheets: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Google.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to log to sheets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Sheets logging failed. Status: %d, Body: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("sheets logging failed with status: %d", resp.StatusCode)
	}

	fmt.Printf("Successfully logged to Google Sheets: %s\n", status)
	return nil
}

func (c *Client) ensureValidToken() error {
	if c.config.Google.TokenExpiry == "" {
		return nil // No expiry set, assume token is valid
	}

	expiry, err := time.Parse(time.RFC3339, c.config.Google.TokenExpiry)
	if err != nil {
		return nil // Can't parse expiry, assume token is valid
	}

	// Refresh token if it expires within 5 minutes
	if time.Until(expiry) < 5*time.Minute {
		return c.RefreshToken()
	}

	return nil
}
