package unit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	repository "origin-service/repository"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStorage создает новый MinIOStorage, подключенный к тестовому экземпляру MinIO.
// Он создает уникальный bucket для изоляции тестов и регистрирует очистку.
func newTestStorage(t *testing.T) *repository.MinIOStorage {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9002" // по умолчанию из compose, если запускается локально
	}
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "testuser"
	}
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = "testpassword"
	}
	bucket := fmt.Sprintf("test-bucket-%d", time.Now().UnixNano())
	useSSL := false // minio-test запускается без SSL

	ctx := context.Background()
	storage, err := repository.NewMinIOStorage(ctx, endpoint, accessKey, secretKey, bucket, useSSL)
	require.NoError(t, err, "failed to create MinIOStorage")

	exists, err := storage.CheckBucketExists(ctx, bucket)
	require.NoError(t, err, "failed to check bucket existence")
	if !exists {
		err = storage.CreateBucket(ctx, bucket)
		require.NoError(t, err, "failed to create bucket")
	}

	// Очистка: удалить bucket и все его содержимое после теста
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Удалить все объекты в bucket
		objectsCh := storage.Client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
		for obj := range objectsCh {
			if obj.Err != nil {
				t.Logf("failed to list object for cleanup: %v", obj.Err)
				continue
			}
			_ = storage.Client.RemoveObject(ctx, bucket, obj.Key, minio.RemoveObjectOptions{})
		}
		// Удалить bucket
		err := storage.Client.RemoveBucket(ctx, bucket)
		if err != nil {
			t.Logf("failed to remove bucket %s: %v", bucket, err)
		}
	})

	return storage
}

// TestUploadFile проверяет успешную загрузку файла в хранилище
func TestUploadFile(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()
	userID := "user123"
	fileName := "test.txt"
	contentType := "text/plain"
	content := "hello world"
	reader := strings.NewReader(content)
	size := int64(len(content))

	// Действие
	metadata, err := repo.UploadFile(ctx, userID, fileName, contentType, reader, size)

	// Проверка
	require.NoError(t, err)
	assert.NotEmpty(t, metadata.FileID)
	assert.Equal(t, userID, metadata.UserID)
	assert.Equal(t, fileName, metadata.FileName)
	assert.Equal(t, contentType, metadata.ContentType)
	assert.Equal(t, size, metadata.Size)
	assert.False(t, metadata.UploadedAt.IsZero())
	// ObjectName должен быть в формате userID/fileID/fileName
	expectedPrefix := userID + "/" + metadata.FileID + "/"
	assert.Contains(t, metadata.ObjectName, expectedPrefix)
	assert.True(t, strings.HasSuffix(metadata.ObjectName, fileName))

	// Проверка, что файл можно получить
	storedFile, err := repo.GetFile(ctx, metadata.FileID)
	require.NoError(t, err)
	assert.Equal(t, metadata, &storedFile.Metadata)

	// Читаем содержимое из Reader и закрываем
	data, err := io.ReadAll(storedFile.Reader)
	require.NoError(t, err)
	storedFile.Reader.Close()
	assert.Equal(t, []byte(content), data)
}

// TestGetFileExisting проверяет получение существующего файла
func TestGetFileExisting(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()
	userID := "user456"
	fileName := "data.bin"
	contentType := "application/octet-stream"
	data := []byte{0x01, 0x02, 0x03}
	reader := bytes.NewReader(data)

	// Загрузка файла для последующего тестирования
	uploadedMeta, err := repo.UploadFile(ctx, userID, fileName, contentType, reader, int64(len(data)))
	require.NoError(t, err)

	// Действие
	stored, err := repo.GetFile(ctx, uploadedMeta.FileID)

	// Проверка
	require.NoError(t, err)
	assert.Equal(t, *uploadedMeta, stored.Metadata)

	// Читаем содержимое из Reader и закрываем
	readData, err := io.ReadAll(stored.Reader)
	require.NoError(t, err)
	stored.Reader.Close()
	assert.Equal(t, data, readData)
}

// TestGetFileNotFound проверяет получение несуществующего файла
func TestGetFileNotFound(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()

	// Действие
	_, err := repo.GetFile(ctx, "nonexistent")

	// Проверка
	assert.ErrorIs(t, err, repository.ErrFileNotFound)
}

// TestGetFileEmptyID проверяет получение файла с пустым идентификатором
func TestGetFileEmptyID(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()

	// Действие
	_, err := repo.GetFile(ctx, "")

	// Проверка
	assert.ErrorContains(t, err, "fileID is required")
}

// TestListFilesByUserIDExisting проверяет получение списка файлов существующего пользователя
func TestListFilesByUserIDExisting(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()
	userA := "userA"

	// Загрузка файлов для пользователя
	metaA1, err := repo.UploadFile(ctx, userA, "a1.txt", "text/plain", strings.NewReader("A1"), 2)
	require.NoError(t, err)
	metaA2, err := repo.UploadFile(ctx, userA, "a2.txt", "text/plain", strings.NewReader("A2"), 2)
	require.NoError(t, err)

	// Действие
	listA, err := repo.ListFilesByUserID(userA)

	// Проверка
	require.NoError(t, err)
	assert.Len(t, listA, 2)
	// порядок может быть разным
	expectedIDs := []string{metaA1.FileID, metaA2.FileID}
	foundIDs := make([]string, len(listA))
	for i, meta := range listA {
		foundIDs[i] = meta.FileID
	}
	assert.ElementsMatch(t, expectedIDs, foundIDs)
}

// TestListFilesByUserIDMultipleUsers проверяет изоляцию файлов между разными пользователями
func TestListFilesByUserIDMultipleUsers(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()
	userA := "userA"
	userB := "userB"

	// Загрузка файлов для разных пользователей
	_, err := repo.UploadFile(ctx, userA, "a1.txt", "text/plain", strings.NewReader("A1"), 2)
	require.NoError(t, err)
	metaB, err := repo.UploadFile(ctx, userB, "b1.txt", "text/plain", strings.NewReader("B1"), 2)
	require.NoError(t, err)

	// Действие
	listB, err := repo.ListFilesByUserID(userB)

	// Проверка
	require.NoError(t, err)
	assert.Len(t, listB, 1)
	assert.Equal(t, metaB.FileID, listB[0].FileID)
}

// TestListFilesByUserIDNonExistent проверяет получение списка файлов несуществующего пользователя
func TestListFilesByUserIDNonExistent(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	// Действие
	listNone, err := repo.ListFilesByUserID("unknown")

	// Проверка
	require.NoError(t, err)
	assert.Empty(t, listNone)
}

// TestListFilesByUserIDEmpty проверяет получение списка файлов с пустым идентификатором пользователя
func TestListFilesByUserIDEmpty(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	// Действие
	_, err := repo.ListFilesByUserID("")

	// Проверка
	assert.ErrorContains(t, err, "userID is required")
}

// TestUploadFileValidationEmptyUserID проверяет валидацию при пустом userID
func TestUploadFileValidationEmptyUserID(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()

	// Действие
	_, err := repo.UploadFile(ctx, "", "file.txt", "text/plain", strings.NewReader("data"), 4)

	// Проверка
	assert.ErrorContains(t, err, "userID is required")
}

// TestUploadFileValidationEmptyFileName проверяет валидацию при пустом имени файла
func TestUploadFileValidationEmptyFileName(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()

	// Действие
	_, err := repo.UploadFile(ctx, "user", "", "text/plain", strings.NewReader("data"), 4)

	// Проверка
	assert.ErrorContains(t, err, "fileName is required")
}

// TestUploadFileValidationNegativeSize проверяет валидацию при отрицательном размере файла
func TestUploadFileValidationNegativeSize(t *testing.T) {
	// Настройка
	storage := newTestStorage(t)
	repo := repository.NewFileRepository(storage)

	ctx := context.Background()

	// Действие
	_, err := repo.UploadFile(ctx, "user", "file.txt", "text/plain", strings.NewReader("data"), -1)

	// Проверка
	assert.ErrorContains(t, err, "size must be >= 0")
}