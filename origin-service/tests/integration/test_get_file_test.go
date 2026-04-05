package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	cacheControlSMaxAge = regexp.MustCompile(`\bs-maxage=(\d+)`)
	cacheControlMaxAge  = regexp.MustCompile(`\bmax-age=(\d+)`)
)

// integrationTestClient возвращает HTTP-клиент с таймаутом для интеграционных тестов.
func integrationTestClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// requireIntegrationAuth возвращает базовый URL API и JWT; падает, если токен не задан.
func requireIntegrationAuth(t *testing.T) (baseURL, jwt string) {
	t.Helper()
	baseURL = getBaseURL()
	jwt = getJWT()
	require.NotEmpty(t, jwt, "JWT token must be provided for test")
	return baseURL, jwt
}

// uploadedFileMeta — поля ответа POST /files, нужные для тестов скачивания.
type uploadedFileMeta struct {
	FileID      string
	ContentType string
}

// mustUploadFile выполняет POST /files и возвращает метаданные при статусе 201.
func mustUploadFile(t *testing.T, baseURL, jwt, fileName string, content []byte) uploadedFileMeta {
	t.Helper()
	uploadURL := baseURL + "/files"
	reqUpload, writer, err := createMultipartRequest(http.MethodPost, uploadURL, "file", fileName, bytes.NewReader(content))
	require.NoError(t, err)
	defer writer.Close()
	reqUpload.Header.Set("Authorization", "Bearer "+jwt)

	respUpload, err := integrationTestClient().Do(reqUpload)
	require.NoError(t, err)
	defer respUpload.Body.Close()
	require.Equal(t, http.StatusCreated, respUpload.StatusCode)

	var meta struct {
		FileID      string `json:"file_id"`
		ContentType string `json:"content_type"`
	}
	require.NoError(t, json.NewDecoder(respUpload.Body).Decode(&meta))
	require.NotEmpty(t, meta.FileID)
	return uploadedFileMeta{FileID: meta.FileID, ContentType: meta.ContentType}
}

// fileDownloadURL собирает публичный URL GET /files/{file_id}.
func fileDownloadURL(baseURL, fileID string) string {
	return baseURL + "/files/" + fileID
}

// placeholderMissingFileID — 32 hex-символа как у настоящего id; объект в хранилище не создаётся.
func placeholderMissingFileID() string {
	return "00000000000000000000000000000000"
}

// mustGETFile200AndETag делает первый GET без заголовков условного запроса: проверяет 200, тело и возвращает ETag.
func mustGETFile200AndETag(t *testing.T, client *http.Client, downloadURL string, expectBody []byte) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expectBody, body)

	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)
	return etag
}

func assertCacheControlDownloadContract(t *testing.T, cc string) {
	t.Helper()
	assert.Contains(t, cc, "public", "Cache-Control must include public")

	sm := cacheControlSMaxAge.FindStringSubmatch(cc)
	require.Len(t, sm, 2, "Cache-Control must contain s-maxage=<seconds>")
	sMax, err := strconv.Atoi(sm[1])
	require.NoError(t, err)
	assert.Greater(t, sMax, 0, "s-maxage must be positive")

	ma := cacheControlMaxAge.FindStringSubmatch(cc)
	require.Len(t, ma, 2, "Cache-Control must contain max-age=<seconds>")
	maxAge, err := strconv.Atoi(ma[1])
	require.NoError(t, err)
	assert.Greater(t, maxAge, 0, "max-age must be positive")
}

// TestDownloadFileSuccess проверяет GET /files/{file_id} без авторизации: тело, Content-Type, Cache-Control, ETag.
func TestDownloadFileSuccess(t *testing.T) {
	// Настройка
	baseURL, jwt := requireIntegrationAuth(t)
	fileContent := []byte("integration get contract body")
	uploaded := mustUploadFile(t, baseURL, jwt, "download-test.bin", fileContent)
	client := integrationTestClient()
	downloadURL := fileDownloadURL(baseURL, uploaded.FileID)
	reqDownload, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	require.NoError(t, err)
	// публичный endpoint — без Authorization

	// Действие
	respDownload, err := client.Do(reqDownload)
	require.NoError(t, err)
	defer respDownload.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusOK, respDownload.StatusCode)

	body, err := io.ReadAll(respDownload.Body)
	require.NoError(t, err)
	assert.Equal(t, fileContent, body, "response body must be raw file bytes")

	assert.Equal(t, uploaded.ContentType, respDownload.Header.Get("Content-Type"))

	cc := respDownload.Header.Get("Cache-Control")
	require.NotEmpty(t, cc)
	assertCacheControlDownloadContract(t, cc)

	etag := respDownload.Header.Get("ETag")
	require.NotEmpty(t, etag, "ETag header must be present")
}

// TestDownloadFileConditionalNotModified проверяет If-None-Match → 304 без тела.
func TestDownloadFileConditionalNotModified(t *testing.T) {
	// Настройка
	baseURL, jwt := requireIntegrationAuth(t)
	fileContent := []byte("etag conditional payload")
	uploaded := mustUploadFile(t, baseURL, jwt, "conditional.bin", fileContent)
	client := integrationTestClient()
	downloadURL := fileDownloadURL(baseURL, uploaded.FileID)
	etag := mustGETFile200AndETag(t, client, downloadURL, fileContent)

	reqConditional, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	require.NoError(t, err)
	reqConditional.Header.Set("If-None-Match", etag)

	// Действие
	respConditional, err := client.Do(reqConditional)
	require.NoError(t, err)
	defer respConditional.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusNotModified, respConditional.StatusCode)

	body304, err := io.ReadAll(respConditional.Body)
	require.NoError(t, err)
	assert.Empty(t, body304, "304 must not include a body")

	assert.Equal(t, etag, respConditional.Header.Get("ETag"))
	cc := respConditional.Header.Get("Cache-Control")
	require.NotEmpty(t, cc)
	assertCacheControlDownloadContract(t, cc)
}

// TestDownloadFileIfNoneMatchMismatch проверяет, что при другом ETag возвращается 200 с полным телом.
func TestDownloadFileIfNoneMatchMismatch(t *testing.T) {
	// Настройка
	baseURL, jwt := requireIntegrationAuth(t)
	fileContent := []byte("mismatch test")
	uploaded := mustUploadFile(t, baseURL, jwt, "mismatch.bin", fileContent)
	client := integrationTestClient()
	downloadURL := fileDownloadURL(baseURL, uploaded.FileID)

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	require.NoError(t, err)
	req.Header.Set("If-None-Match", `"definitely-not-the-server-etag"`)

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, fileContent, body)
}

// TestDownloadFileNotFound проверяет 404 для несуществующего file_id.
func TestDownloadFileNotFound(t *testing.T) {
	// Настройка
	baseURL := getBaseURL()
	url := fileDownloadURL(baseURL, placeholderMissingFileID())

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	client := integrationTestClient()

	// Действие
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверка
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
