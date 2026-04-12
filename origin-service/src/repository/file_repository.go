package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	minio "github.com/minio/minio-go/v7"
)

type FileRepository struct {
	db *MinIOStorage
}

type FileStream struct {
	Metadata FileMetadata
	Reader   io.ReadCloser
}

func NewFileRepository(db *MinIOStorage) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) GetFile(ctx context.Context, fileID string) (*FileStream, error) {
	if fileID == "" {
		slog.Warn("get file called with empty fileID")
		return nil, fmt.Errorf("fileID is required")
	}

	meta, err := r.db.loadMetadata(ctx, fileID)
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			slog.Warn("file metadata not found", "fileID", fileID)
		} else {
			slog.Error("failed to load metadata", "error", err, "fileID", fileID)
		}
		return nil, ErrFileNotFound
	}

	obj, err := r.db.Client.GetObject(ctx, r.db.bucket, meta.ObjectName, minio.GetObjectOptions{})
	if err != nil {
		slog.Error("failed to get object from MinIO", "error", err, "fileID", fileID, "object", meta.ObjectName)
		return nil, fmt.Errorf("get object: %w", err)
	}

	return &FileStream{
		Metadata: meta,
		Reader:   obj, // obj реализует io.ReadCloser
	}, nil
}

func (r *FileRepository) ListFilesByUserID(userID string) ([]FileMetadata, error) {
	if userID == "" {
		slog.Warn("list files called with empty userID")
		return nil, fmt.Errorf("userID is required")
	}

	allMetas, err := r.db.listMetadataByUserID(context.Background(), userID)
	if err != nil {
		slog.Error("failed to list metadata", "error", err)
		return nil, err
	}
	slog.Debug("listed files", "userID", userID, "count", len(allMetas))
	return allMetas, nil
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
		ObjectName:  buildContentObjectKey(userID, fileID, fileName),
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		UploadedAt:  time.Now().UTC(),
		Checksum:    "",
	}

	// Вычисляем хеш во время отправки в MinIO
	hasher := sha1.New()
	teeReader := io.TeeReader(reader, hasher)

	_, err = r.db.Client.PutObject(ctx, r.db.bucket, meta.ObjectName, teeReader, meta.Size, minio.PutObjectOptions{
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

	// Сохраняем хеш
	meta.Checksum = hex.EncodeToString(hasher.Sum(nil))
	slog.Info("calculated checksum", "fileID", fileID, "checksum", meta.Checksum)

	if err := r.db.saveMetadata(ctx, meta); err != nil {
		slog.Error("failed to save metadata, attempting to delete uploaded object and metadata keys",
			"error", err, "object", meta.ObjectName)

		_ = r.db.Client.RemoveObject(ctx, r.db.bucket, meta.ObjectName, minio.RemoveObjectOptions{})
		_ = r.db.Client.RemoveObject(ctx, r.db.bucket, buildSidecarMetaKey(userID, fileID), minio.RemoveObjectOptions{})
		_ = r.db.Client.RemoveObject(ctx, r.db.bucket, buildFileIndexKey(fileID), minio.RemoveObjectOptions{})
		return nil, fmt.Errorf("save metadata: %w", err)
	}

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
