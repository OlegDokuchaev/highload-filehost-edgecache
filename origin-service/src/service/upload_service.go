package service

import (
	"context"
	"fmt"
	"io"
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
	Bytes       []byte
	ContentType string
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
		return nil, fmt.Errorf("userID is required")
	}
	if input.FileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if input.Body == nil {
		return nil, fmt.Errorf("file body is required")
	}
	if input.SizeBytes > s.maxUploadSize {
		return nil, ErrPayloadTooLarge
	}
	contentType := input.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta, err := s.repo.UploadFile(
		ctx,
		input.UserID,
		input.FileName,
		contentType,
		input.Body,
		input.SizeBytes,
	)
	if err != nil {
		return nil, err
	}

	return mapFileMetadata(meta), nil
}

func (s *UploadService) ListByUserID(userID string) ([]FileInfo, error) {
	metas, err := s.repo.ListFilesByUserID(userID)
	if err != nil {
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
		return nil, ErrNotFound
	}

	stored, err := s.repo.GetFile(ctx, fileID)
	if err != nil {
		if err == repository.ErrFileNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	contentType := stored.Metadata.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &FileContent{
		Bytes:       stored.Data,
		ContentType: contentType,
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
