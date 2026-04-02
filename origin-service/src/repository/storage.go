package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
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

// metaUserMap сериализует поля в пользовательские метаданные объекта S3/MinIO.
func metaUserMap(meta FileMetadata) map[string]string {
	m := map[string]string{
		"fileid":      meta.FileID,
		"userid":      meta.UserID,
		"filename":    meta.FileName,
		"contenttype": meta.ContentType,
		"uploadedat":  meta.UploadedAt.UTC().Format(time.RFC3339Nano),
		"size":        strconv.FormatInt(meta.Size, 10),
	}
	if meta.Checksum != "" {
		m["checksum"] = meta.Checksum
	}
	return m
}

func userMetaGet(um map[string]string, canonical string) string {
	if um == nil {
		return ""
	}
	for k, v := range um {
		if strings.EqualFold(k, canonical) {
			return v
		}
	}
	return ""
}

func parseKeySegments(objectKey string) (userID, fileID, fileName string, ok bool) {
	parts := strings.SplitN(objectKey, "/", 3)
	if len(parts) < 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func fileMetadataFromObjectInfo(info minio.ObjectInfo) (FileMetadata, error) {
	um := map[string]string(info.UserMetadata)

	userID, fileID, fileName, _ := parseKeySegments(info.Key)
	if id := userMetaGet(um, "fileid"); id != "" {
		fileID = id
	}
	if id := userMetaGet(um, "userid"); id != "" {
		userID = id
	}
	if fn := userMetaGet(um, "filename"); fn != "" {
		fileName = fn
	}

	if fileID == "" || userID == "" || fileName == "" {
		return FileMetadata{}, fmt.Errorf("incomplete metadata for object %q", info.Key)
	}

	contentType := info.ContentType
	if ct := userMetaGet(um, "contenttype"); ct != "" {
		contentType = ct
	}

	size := info.Size
	if s := userMetaGet(um, "size"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			size = n
		}
	}

	uploadedAt := info.LastModified
	if ts := userMetaGet(um, "uploadedat"); ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			uploadedAt = t
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			uploadedAt = t
		}
	}

	return FileMetadata{
		FileID:      fileID,
		UserID:      userID,
		ObjectName:  info.Key,
		FileName:    fileName,
		ContentType: contentType,
		Size:        size,
		UploadedAt:  uploadedAt,
		Checksum:    userMetaGet(um, "checksum"),
	}, nil
}

func (s *MinIOStorage) findObjectKeyByFileID(ctx context.Context, fileID string) (string, error) {
	objects := s.Client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Recursive: true})
	for obj := range objects {
		if obj.Err != nil {
			return "", fmt.Errorf("list objects: %w", obj.Err)
		}
		if strings.HasPrefix(obj.Key, "metadata/") {
			continue
		}
		_, id, _, ok := parseKeySegments(obj.Key)
		if ok && id == fileID {
			return obj.Key, nil
		}
	}
	return "", ErrFileNotFound
}

func (s *MinIOStorage) copyObjectWithUserMeta(ctx context.Context, objectKey string, meta FileMetadata) error {
	_, err := s.Client.CopyObject(ctx,
		minio.CopyDestOptions{
			Bucket:          s.bucket,
			Object:          objectKey,
			UserMetadata:    metaUserMap(meta),
			ReplaceMetadata: true,
		},
		minio.CopySrcOptions{
			Bucket: s.bucket,
			Object: objectKey,
		},
	)
	if err != nil {
		return fmt.Errorf("copy object (metadata refresh): %w", err)
	}
	return nil
}

// listFileMetadataByUserPrefix возвращает метаданные объектов с ключом userID/...
func (s *MinIOStorage) listFileMetadataByUserPrefix(ctx context.Context, userID string) ([]FileMetadata, error) {
	prefix := userID + "/"
	objects := s.Client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	var metas []FileMetadata
	for obj := range objects {
		if obj.Err != nil {
			return nil, fmt.Errorf("list objects: %w", obj.Err)
		}
		if obj.Key == "" || strings.HasSuffix(obj.Key, "/") {
			continue
		}
		info, err := s.Client.StatObject(ctx, s.bucket, obj.Key, minio.StatObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("stat object %s: %w", obj.Key, err)
		}
		meta, err := fileMetadataFromObjectInfo(info)
		if err != nil {
			slog.Warn("skip object with invalid metadata", "key", obj.Key, "error", err)
			continue
		}
		if meta.UserID != userID {
			continue
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
