package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
)

type FileRepository struct {
	db *MinIOStorage
}

func NewFileRepository(db *MinIOStorage) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) GetFile(ctx context.Context, fileID string) (*StoredFile, error) {
	if fileID == "" {
		slog.Warn("get file called with empty fileID")
		return nil, fmt.Errorf("fileID is required")
	}

	r.db.mu.RLock()
	meta, ok := r.db.metadata[fileID]
	r.db.mu.RUnlock()
	if !ok {
		slog.Warn("file metadata not found", "fileID", fileID)
		return nil, ErrFileNotFound
	}

	obj, err := r.db.Client.GetObject(ctx, r.db.bucket, meta.ObjectName, minio.GetObjectOptions{})
	if err != nil {
		slog.Error("failed to get object from MinIO", "error", err, "fileID", fileID, "object", meta.ObjectName)
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		slog.Error("failed to read object body", "error", err, "fileID", fileID)
		return nil, fmt.Errorf("read object body: %w", err)
	}

	return &StoredFile{
		Metadata: meta,
		Data:     data,
	}, nil
}

func (r *FileRepository) ListFilesByUserID(userID string) ([]FileMetadata, error) {
	if userID == "" {
		slog.Warn("list files called with empty userID")
		return nil, fmt.Errorf("userID is required")
	}

	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	items := make([]FileMetadata, 0)
	for _, meta := range r.db.metadata {
		if meta.UserID == userID {
			items = append(items, meta)
		}
	}
	slog.Debug("listed files", "userID", userID, "count", len(items))
	return items, nil
}

func (r *FileRepository) UploadFile(
	ctx context.Context,
	userID string,
	fileName string,
	contentType string,
	reader io.Reader,
	size int64,
) (*FileMetadata, error) {
	if userID == "" {
		slog.Warn("upload file called with empty userID")
		return nil, fmt.Errorf("userID is required")
	}
	if fileName == "" {
		slog.Warn("upload file called with empty fileName")
		return nil, fmt.Errorf("fileName is required")
	}
	if size < 0 {
		slog.Warn("upload file called with negative size")
		return nil, fmt.Errorf("size must be >= 0")
	}

	fileID, err := generateFileID()
	if err != nil {
		slog.Error("failed to generate fileID", "error", err)
		return nil, err
	}

	meta := FileMetadata{
		FileID:      fileID,
		UserID:      userID,
		ObjectName:  buildObjectName(userID, fileID, fileName),
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		UploadedAt:  time.Now().UTC(),
	}

	_, err = r.db.Client.PutObject(ctx, r.db.bucket, meta.ObjectName, reader, meta.Size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"fileId": meta.FileID,
			"userId": meta.UserID,
		},
	})
	if err != nil {
		slog.Error("failed to put object to MinIO", "error", err, "fileID", fileID, "object", meta.ObjectName)
		return nil, fmt.Errorf("put object: %w", err)
	}

	r.db.mu.Lock()
	r.db.metadata[meta.FileID] = meta
	r.db.mu.Unlock()

	slog.Info("file uploaded to repository", "fileID", fileID, "userID", userID, "object", meta.ObjectName)
	return &meta, nil
}

func generateFileID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate file id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}