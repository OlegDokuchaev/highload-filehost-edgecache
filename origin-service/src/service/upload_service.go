package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"origin-service/repository"
)

const defaultMaxUploadSizeBytes int64 = 50 * 1024 * 1024

var (
	ErrPayloadTooLarge = fmt.Errorf("payload too large")
	ErrNotFound        = fmt.Errorf("not found")
)

type UploadInput struct {
	UserID      string
	FileName    string
	ContentType string
	SizeBytes   int64
	Body        io.Reader
}

type FileInfo struct {
	FileID      string
	OwnerUserID string
	FileName    string
	ContentType string
	SizeBytes   int64
	CreatedAt   time.Time
}

type FileContent struct {
    Reader      io.ReadCloser
    ContentType string
    Size        int64
    ETag        string
}

type UploadService struct {
	repo          *repository.FileRepository
	maxUploadSize int64
}

func NewUploadService(repo *repository.FileRepository) *UploadService {
	return &UploadService{
		repo:          repo,
		maxUploadSize: defaultMaxUploadSizeBytes,
	}
}

func (s *UploadService) Upload(ctx context.Context, input UploadInput) (*FileInfo, error) {
	if input.UserID == "" {
		slog.Warn("upload attempt with empty userID")
		return nil, fmt.Errorf("userID is required")
	}
	if input.FileName == "" {
		slog.Warn("upload attempt with empty fileName")
		return nil, fmt.Errorf("fileName is required")
	}
	if input.Body == nil {
		slog.Warn("upload attempt with nil body")
		return nil, fmt.Errorf("file body is required")
	}
	if input.SizeBytes > s.maxUploadSize {
		slog.Warn("upload rejected: payload too large", "size", input.SizeBytes, "max", s.maxUploadSize)
		return nil, ErrPayloadTooLarge
	}
	contentType := input.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	slog.Info("upload started", "userID", input.UserID, "fileName", input.FileName, "size", input.SizeBytes)

	meta, err := s.repo.UploadFile(
		ctx,
		input.UserID,
		input.FileName,
		contentType,
		input.Body,
		input.SizeBytes,
	)
	if err != nil {
		slog.Error("upload failed", "error", err, "userID", input.UserID, "fileName", input.FileName)
		return nil, err
	}

	slog.Info("upload completed", "fileID", meta.FileID, "userID", meta.UserID, "fileName", meta.FileName)
	return mapFileMetadata(meta), nil
}

func (s *UploadService) ListByUserID(userID string) ([]FileInfo, error) {
	if userID == "" {
		slog.Warn("list files called with empty userID")
		return nil, fmt.Errorf("userID is required")
	}
	metas, err := s.repo.ListFilesByUserID(userID)
	if err != nil {
		slog.Error("failed to list files", "error", err, "userID", userID)
		return nil, err
	}
	items := make([]FileInfo, 0, len(metas))
	for i := range metas {
		items = append(items, *mapFileMetadata(&metas[i]))
	}
	return items, nil
}

func (s *UploadService) DownloadByFileID(ctx context.Context, fileID string) (*FileContent, error) {
    if fileID == "" {
        slog.Warn("download called with empty fileID")
        return nil, ErrNotFound
    }

    stream, err := s.repo.GetFile(ctx, fileID)
    if err != nil {
        if err == repository.ErrFileNotFound {
            slog.Warn("file not found", "fileID", fileID)
            return nil, ErrNotFound
        }
        slog.Error("failed to get file", "error", err, "fileID", fileID)
        return nil, err
    }

    contentType := stream.Metadata.ContentType
    if contentType == "" {
        contentType = "application/octet-stream"
    }

    // Формируем ETag
    var etag string
    if stream.Metadata.Checksum != "" {
        // etag = `"` + stream.Metadata.Checksum + `"`
        etag = stream.Metadata.Checksum
    } else {
        // Слабый ETag для старых файлов
        etag = fmt.Sprintf(`W/"%d-%d"`, stream.Metadata.Size, stream.Metadata.UploadedAt.Unix())
    }

    return &FileContent{
        Reader:      stream.Reader,
        ContentType: contentType,
        Size:        stream.Metadata.Size,
        ETag:        etag,
    }, nil
}

func mapFileMetadata(meta *repository.FileMetadata) *FileInfo {
	return &FileInfo{
		FileID:      meta.FileID,
		OwnerUserID: meta.UserID,
		FileName:    meta.FileName,
		ContentType: meta.ContentType,
		SizeBytes:   meta.Size,
		CreatedAt:   meta.UploadedAt,
	}
}