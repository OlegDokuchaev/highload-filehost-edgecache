package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// API_BASE_URL      - адрес тестируемого сервиса (например, http://localhost:8080)
// JWT_TOKEN         - валидный JWT-токен для авторизованных запросов
// LARGE_FILE_SIZE   - (опционально) размер файла для проверки ошибки 413, по умолчанию 10 МБ

const (
	testJWT = "test-token-user-1" // <-- валидный JWT-токен для тестов
)

func getBaseURL() string {
	baseURL := os.Getenv("API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://origin-service-test:8080" // Значение по умолчанию
	}
	return baseURL
}

func getJWT() string {
	if testJWT != "" {
		return testJWT
	}
	return os.Getenv("JWT_TOKEN")
}

func getLargeFileSize() int64 {
	const defaultSize = 101 * 1024 * 1024 // 101 МБ
	if sizeStr := os.Getenv("LARGE_FILE_SIZE"); sizeStr != "" {
		var size int64
		fmt.Sscanf(sizeStr, "%d", &size)
		if size > 0 {
			return size
		}
	}
	return defaultSize
}

// createMultipartRequest создаёт HTTP-запрос с multipart/form-data, содержащим один файл.
// Возвращает запрос и writer, который необходимо закрыть после записи тела.
func createMultipartRequest(method, url, fieldName, fileName string, fileContent io.Reader) (*http.Request, *multipart.Writer, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, nil, err
	}
	if _, err := io.Copy(part, fileContent); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer, nil
}

// TestUploadFileSuccess проверяет успешную загрузку файла.
func TestUploadFileSuccess(t *testing.T) {
	// Настройка
	baseURL := getBaseURL()
	jwtToken := getJWT()
	require.NotEmpty(t, jwtToken, "JWT token must be provided for test")

	url := baseURL + "/files"
	fileContent := []byte("hello world")
	reader := bytes.NewReader(fileContent)

	req, writer, err := createMultipartRequest(http.MethodPost, url, "file", "test.txt", reader)
	require.NoError(t, err)
	defer writer.Close()

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	client := &http.Client{Timeout: 10 * time.Second}

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var response struct {
		FileID      string    `json:"file_id"`
		OwnerUserID string    `json:"owner_user_id"`
		Filename    string    `json:"filename"`
		ContentType string    `json:"content_type"`
		SizeBytes   int64     `json:"size_bytes"`
		CreatedAt   time.Time `json:"created_at"`
		DownloadURL string    `json:"download_url"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.FileID)
	assert.NotEmpty(t, response.OwnerUserID)
	assert.Equal(t, "test.txt", response.Filename)
	assert.Equal(t, "application/octet-stream", response.ContentType)
	assert.Equal(t, int64(len(fileContent)), response.SizeBytes)
	assert.False(t, response.CreatedAt.IsZero())
	assert.Contains(t, response.DownloadURL, response.FileID)
}

// TestUploadFileMissingFile проверяет ошибку при отсутствии файла в запросе.
func TestUploadFileMissingFile(t *testing.T) {
	// Настройка
	baseURL := getBaseURL()
	jwtToken := getJWT()
	require.NotEmpty(t, jwtToken, "JWT token must be provided for test")

	url := baseURL + "/files"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, url, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	client := &http.Client{Timeout: 10 * time.Second}

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(bodyBytes), "file")
}

// TestUploadFileInvalidToken проверяет ошибку при невалидном JWT.
func TestUploadFileInvalidToken(t *testing.T) {
	// Настройка
	baseURL := getBaseURL()
	url := baseURL + "/files"
	fileContent := []byte("hello world")
	reader := bytes.NewReader(fileContent)

	req, writer, err := createMultipartRequest(http.MethodPost, url, "file", "test.txt", reader)
	require.NoError(t, err)
	defer writer.Close()

	req.Header.Set("Authorization", "Bearer invalid-token")
	client := &http.Client{Timeout: 10 * time.Second}

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestUploadFileLargePayload проверяет ошибку 413 Payload Too Large.
// Для этого теста необходимо, чтобы на тестовом стенде был настроен лимит размера запроса.
// Размер отправляемого файла берётся из переменной окружения LARGE_FILE_SIZE (по умолчанию 10 МБ).
func TestUploadFileLargePayload(t *testing.T) {
	// Настройка
	baseURL := getBaseURL()
	jwtToken := getJWT()
	require.NotEmpty(t, jwtToken, "JWT token must be provided for test")

	url := baseURL + "/files"
	size := getLargeFileSize()
	largeContent := make([]byte, size)
	reader := bytes.NewReader(largeContent)

	req, writer, err := createMultipartRequest(http.MethodPost, url, "file", "large.bin", reader)
	require.NoError(t, err)
	defer writer.Close()

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	client := &http.Client{Timeout: 30 * time.Second}

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Logf("Unexpected status code: %d, maybe the limit is higher", resp.StatusCode)
	}
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "Expected 413, got %d", resp.StatusCode)
}
