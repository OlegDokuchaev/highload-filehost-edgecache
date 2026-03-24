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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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
}

type StoredFile struct {
	Metadata FileMetadata
	Data     []byte
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

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		slog.Warn("failed to check bucket existence", "error", err)
		// return nil, fmt.Errorf("check bucket existence: %w", err)
	}
	if !exists {
		if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			slog.Error("failed to create bucket", "error", err)
			return nil, fmt.Errorf("create bucket: %w", err)
		}
		slog.Info("created bucket", "bucket", bucket)
	}

	return &MinIOStorage{
		Client: client,
		bucket: bucket,
	}, nil
}

func (s *MinIOStorage) saveMetadata(ctx context.Context, meta FileMetadata) error {
	key := fmt.Sprintf("metadata/%s.json", meta.FileID)
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.Client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return fmt.Errorf("put metadata object: %w", err)
	}
	return nil
}

func (s *MinIOStorage) loadMetadata(ctx context.Context, fileID string) (FileMetadata, error) {
	key := fmt.Sprintf("metadata/%s.json", fileID)
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

func (s *MinIOStorage) listAllMetadata(ctx context.Context) ([]FileMetadata, error) {
	var metas []FileMetadata
	objects := s.Client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    "metadata/",
		Recursive: true,
	})
	for obj := range objects {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
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

func buildObjectName(userID, fileID, fileName string) string {
	cleanName := strings.ReplaceAll(fileName, "/", "_")
	cleanName = strings.ReplaceAll(cleanName, "\\", "_")
	return fmt.Sprintf("%s/%s/%s", userID, fileID, cleanName)
}