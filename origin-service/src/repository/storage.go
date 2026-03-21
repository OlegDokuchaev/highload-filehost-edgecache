package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrFileNotFound = errors.New("file not found")

type FileMetadata struct {
	FileID     string
	UserID     string
	ObjectName string
	FileName   string
	Size       int64
	UploadedAt time.Time
}

type StoredFile struct {
	Metadata FileMetadata
	Data     []byte
}

type MinIOStorage struct {
	Client   *minio.Client
	bucket   string
	metadata map[string]FileMetadata
	mu       sync.RWMutex
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
		return nil, fmt.Errorf("init minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket existence: %w", err)
	}
	if !exists {
		if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	return &MinIOStorage{
		Client:   client,
		bucket:   bucket,
		metadata: make(map[string]FileMetadata),
	}, nil
}

func buildObjectName(userID, fileID, fileName string) string {
	cleanName := strings.ReplaceAll(fileName, "/", "_")
	cleanName = strings.ReplaceAll(cleanName, "\\", "_")
	return fmt.Sprintf("%s/%s/%s", userID, fileID, cleanName)
}
