package unit

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repository "origin-service/repository"
	service "origin-service/service"
)

// newTestUploadService создает новый UploadService, подключенный к тестовому репозиторию.
func newTestUploadService(t *testing.T) *service.UploadService {
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)
	return service.NewUploadService(repo)
}

// TestUpload_Success проверяет успешную загрузку файла.
func TestUpload_Success(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	content := "hello world"
	input := service.UploadInput{
		UserID:      "user123",
		FileName:    "test.txt",
		ContentType: "text/plain",
		SizeBytes:   int64(len(content)),
		Body:        strings.NewReader(content),
	}

	// Действие
	info, err := svc.Upload(ctx, input)

	// Проверка
	require.NoError(t, err)
	assert.NotEmpty(t, info.FileID)
	assert.Equal(t, input.UserID, info.OwnerUserID)
	assert.Equal(t, input.FileName, info.FileName)
	assert.Equal(t, input.ContentType, info.ContentType)
	assert.Equal(t, input.SizeBytes, info.SizeBytes)
	assert.False(t, info.CreatedAt.IsZero())
}

// TestUpload_EmptyUserID проверяет ошибку валидации при пустом UserID.
func TestUpload_EmptyUserID(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()
	input := service.UploadInput{
		UserID:      "",
		FileName:    "file.txt",
		ContentType: "text/plain",
		SizeBytes:   10,
		Body:        strings.NewReader("data"),
	}

	// Действие
	_, err := svc.Upload(ctx, input)

	// Проверка
	assert.ErrorContains(t, err, "userID is required")
}

// TestUpload_EmptyFileName проверяет ошибку валидации при пустом FileName.
func TestUpload_EmptyFileName(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "",
		ContentType: "text/plain",
		SizeBytes:   10,
		Body:        strings.NewReader("data"),
	}

	// Действие
	_, err := svc.Upload(ctx, input)

	// Проверка
	assert.ErrorContains(t, err, "fileName is required")
}

// TestUpload_NilBody проверяет ошибку валидации при nil Body.
func TestUpload_NilBody(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "file.txt",
		ContentType: "text/plain",
		SizeBytes:   10,
		Body:        nil,
	}

	// Действие
	_, err := svc.Upload(ctx, input)

	// Проверка
	assert.ErrorContains(t, err, "file body is required")
}

// TestUpload_DefaultContentType проверяет, что пустой ContentType по умолчанию равен application/octet-stream.
func TestUpload_DefaultContentType(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	content := "hello"
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "file.bin",
		ContentType: "",
		SizeBytes:   int64(len(content)),
		Body:        strings.NewReader(content),
	}

	// Действие
	info, err := svc.Upload(ctx, input)

	// Проверка
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", info.ContentType)
}

// TestUpload_SizeTooLarge проверяет ограничение размера payload.
func TestUpload_SizeTooLarge(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	bigSize := int64(51 * 1024 * 1024)
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "big.txt",
		ContentType: "text/plain",
		SizeBytes:   bigSize,
		Body:        strings.NewReader("data"),
	}

	// Действие
	_, err := svc.Upload(ctx, input)

	// Проверка
	assert.ErrorIs(t, err, service.ErrPayloadTooLarge)
}

// TestListByUserID_Success проверяет список файлов для пользователя.
func TestListByUserID_Success(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()
	userID := "list-user"

	content := "content"
	_, err := svc.Upload(ctx, service.UploadInput{
		UserID:      userID,
		FileName:    "file1.txt",
		ContentType: "text/plain",
		SizeBytes:   int64(len(content)),
		Body:        strings.NewReader(content),
	})
	require.NoError(t, err)

	_, err = svc.Upload(ctx, service.UploadInput{
		UserID:      userID,
		FileName:    "file2.txt",
		ContentType: "text/plain",
		SizeBytes:   int64(len(content)),
		Body:        strings.NewReader(content),
	})
	require.NoError(t, err)

	// Действие
	files, err := svc.ListByUserID(userID)

	// Проверка
	require.NoError(t, err)
	assert.Len(t, files, 2)

	fileNames := make([]string, len(files))
	for i, f := range files {
		fileNames[i] = f.FileName
	}
	assert.ElementsMatch(t, []string{"file1.txt", "file2.txt"}, fileNames)
}

// TestListByUserID_EmptyList проверяет список файлов для пользователя без файлов.
func TestListByUserID_EmptyList(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)

	// Действие
	files, err := svc.ListByUserID("nonexistent-user")

	// Проверка
	require.NoError(t, err)
	assert.Empty(t, files)
}

// TestListByUserID_RepoError проверяет, что ошибки репозитория распространяются.
func TestListByUserID_RepoError(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)

	// Действие
	_, err := svc.ListByUserID("")

	// Проверка
	assert.ErrorContains(t, err, "userID is required")
}

// TestDownloadByFileID_Success проверяет успешную загрузку существующего файла.
func TestDownloadByFileID_Success(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	content := []byte("download me")
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "download.txt",
		ContentType: "text/plain",
		SizeBytes:   int64(len(content)),
		Body:        bytes.NewReader(content),
	}
	info, err := svc.Upload(ctx, input)
	require.NoError(t, err)

	// Действие
	fileContent, err := svc.DownloadByFileID(ctx, info.FileID)

	// Проверка
	require.NoError(t, err)
	assert.Equal(t, content, fileContent.Bytes)
	assert.Equal(t, input.ContentType, fileContent.ContentType)
}

// TestDownloadByFileID_NotFound проверяет, что ErrNotFound возвращается для отсутствующего файла.
func TestDownloadByFileID_NotFound(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	// Действие
	_, err := svc.DownloadByFileID(ctx, "nonexistent")

	// Проверка
	assert.ErrorIs(t, err, service.ErrNotFound)
}

// TestDownloadByFileID_EmptyID проверяет, что пустой fileID возвращает ErrNotFound.
func TestDownloadByFileID_EmptyID(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	// Действие
	_, err := svc.DownloadByFileID(ctx, "")

	// Проверка
	assert.ErrorIs(t, err, service.ErrNotFound)
}

// TestDownloadByFileID_DefaultContentType проверяет, что отсутствующий ContentType в хранилище по умолчанию равен application/octet-stream.
func TestDownloadByFileID_DefaultContentType(t *testing.T) {
	// Настройка
	svc := newTestUploadService(t)
	ctx := context.Background()

	content := []byte("test")
	input := service.UploadInput{
		UserID:      "user",
		FileName:    "noctype.bin",
		ContentType: "",
		SizeBytes:   int64(len(content)),
		Body:        bytes.NewReader(content),
	}
	info, err := svc.Upload(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", info.ContentType)

	// Действие
	fileContent, err := svc.DownloadByFileID(ctx, info.FileID)

	// Проверка
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", fileContent.ContentType)
}