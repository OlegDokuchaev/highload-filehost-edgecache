package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	minio "github.com/minio/minio-go/v7"
	credentials "github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrFileNotFound = errors.New("file not found")

type FileMetadata struct {
	FileID      string
	UserID      string
	ObjectName  string
	FileName    string
	ContentType string
	Size        int64
	UploadedAt  time.Time
	Checksum    string
}

type MinIOStorage struct {
	Client *minio.Client
	bucket string
}

func NewMinIOStorage(
	ctx context.Context,
	endpoint string,
	accessKey string,
	secretKey string,
	bucket string,
	useSSL bool,
) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		slog.Error("failed to create minio client", "error", err)
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	return &MinIOStorage{
		Client: client,
		bucket: bucket,
	}, nil
}

func (s *MinIOStorage) CreateBucket(ctx context.Context, bucket string) error {
	exists, err := s.Client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket existence: %w", err)
	}
	if exists {
		return fmt.Errorf("bucket %s already exists", bucket)
	}
	err = s.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

func (s *MinIOStorage) CheckBucketExists(ctx context.Context, bucket string) (bool, error) {
	exists, err := s.Client.BucketExists(ctx, bucket)
	if err != nil {
		return false, fmt.Errorf("check bucket existence: %w", err)
	}
	return exists, nil
}

func (s *MinIOStorage) saveMetadata(ctx context.Context, meta FileMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	opts := minio.PutObjectOptions{ContentType: "application/json"}
	sidecarKey := buildSidecarMetaKey(meta.UserID, meta.FileID)
	_, err = s.Client.PutObject(ctx, s.bucket, sidecarKey, bytes.NewReader(data), int64(len(data)), opts)
	if err != nil {
		return fmt.Errorf("put sidecar metadata: %w", err)
	}
	indexKey := buildFileIndexKey(meta.FileID)
	_, err = s.Client.PutObject(ctx, s.bucket, indexKey, bytes.NewReader(data), int64(len(data)), opts)
	if err != nil {
		_ = s.Client.RemoveObject(ctx, s.bucket, sidecarKey, minio.RemoveObjectOptions{})
		return fmt.Errorf("put file index metadata: %w", err)
	}
	return nil
}

func (s *MinIOStorage) loadMetadata(ctx context.Context, fileID string) (FileMetadata, error) {
	key := buildFileIndexKey(fileID)
	obj, err := s.Client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return FileMetadata{}, ErrFileNotFound
		}
		return FileMetadata{}, fmt.Errorf("get metadata object: %w", err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return FileMetadata{}, fmt.Errorf("read metadata object: %w", err)
	}
	var meta FileMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return FileMetadata{}, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return meta, nil
}

func (s *MinIOStorage) listMetadataByUserID(ctx context.Context, userID string) ([]FileMetadata, error) {
	prefix := fmt.Sprintf("files/%s/", userID)
	var metas []FileMetadata
	objects := s.Client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for obj := range objects {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}
		if !strings.HasSuffix(obj.Key, "/meta.json") {
			continue
		}
		objReader, err := s.Client.GetObject(ctx, s.bucket, obj.Key, minio.GetObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("get object %s: %w", obj.Key, err)
		}
		data, err := io.ReadAll(objReader)
		objReader.Close()
		if err != nil {
			return nil, fmt.Errorf("read object %s: %w", obj.Key, err)
		}
		var meta FileMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("unmarshal metadata from %s: %w", obj.Key, err)
		}
		metas = append(metas, meta)
	}
	return metas, nil
}

func buildContentObjectKey(userID, fileID, fileName string) string {
	cleanName := strings.ReplaceAll(fileName, "/", "_")
	cleanName = strings.ReplaceAll(cleanName, "\\", "_")
	return fmt.Sprintf("files/%s/%s/%s", userID, fileID, cleanName)
}

func buildSidecarMetaKey(userID, fileID string) string {
	return fmt.Sprintf("files/%s/%s/meta.json", userID, fileID)
}

func buildFileIndexKey(fileID string) string {
	return fmt.Sprintf("files/_index/%s", fileID)
}
