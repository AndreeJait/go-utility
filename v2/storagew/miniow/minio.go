package miniow

import (
	"context"
	"io"
	"time"

	"github.com/AndreeJait/go-utility/v2/storagew"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// minioStorage implements the storagew.Storage interface for MinIO and S3-compatible servers.
type minioStorage struct {
	client *minio.Client
	core   *minio.Core // Core is required for explicit/manual multipart operations
}

// Config holds the credentials and endpoint configuration for MinIO.
type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
}

// New initializes a new MinIO storage client and validates the connection configuration.
func New(cfg *Config) (storagew.Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &minioStorage{
		client: client,
		core:   &minio.Core{Client: client},
	}, nil
}

func (m *minioStorage) Upload(ctx context.Context, bucket, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    10 * 1024 * 1024, // Enable automatic 10MB chunking for large streams
	}
	_, err := m.client.PutObject(ctx, bucket, objectName, reader, objectSize, opts)
	return err
}

func (m *minioStorage) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	return m.client.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
}

func (m *minioStorage) Delete(ctx context.Context, bucket, objectName string) error {
	return m.client.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{})
}

func (m *minioStorage) GetPresignedURL(ctx context.Context, bucket, objectName string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (m *minioStorage) InitMultipartUpload(ctx context.Context, bucket, objectName, contentType string) (string, error) {
	opts := minio.PutObjectOptions{ContentType: contentType}
	return m.core.NewMultipartUpload(ctx, bucket, objectName, opts)
}

func (m *minioStorage) UploadPart(ctx context.Context, bucket, objectName, uploadID string, partNumber int, reader io.Reader, partSize int64) (storagew.MultipartPart, error) {
	part, err := m.core.PutObjectPart(ctx, bucket, objectName, uploadID, partNumber, reader, partSize, minio.PutObjectPartOptions{})
	if err != nil {
		return storagew.MultipartPart{}, err
	}
	return storagew.MultipartPart{
		PartNumber: part.PartNumber,
		ETag:       part.ETag,
	}, nil
}

func (m *minioStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectName, uploadID string, parts []storagew.MultipartPart) error {
	minioParts := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		minioParts[i] = minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}
	_, err := m.core.CompleteMultipartUpload(ctx, bucket, objectName, uploadID, minioParts, minio.PutObjectOptions{})
	return err
}

func (m *minioStorage) AbortMultipartUpload(ctx context.Context, bucket, objectName, uploadID string) error {
	return m.core.AbortMultipartUpload(ctx, bucket, objectName, uploadID)
}
