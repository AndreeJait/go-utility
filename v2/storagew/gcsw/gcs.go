package gcsw

import (
	"context"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"github.com/AndreeJait/go-utility/v2/storagew"
)

// gcsStorage implements the storagew.Storage interface for Google Cloud Storage.
type gcsStorage struct {
	client *storage.Client
}

// New initializes a new Google Cloud Storage client.
// It relies on the standard GOOGLE_APPLICATION_CREDENTIALS environment variable.
func New(ctx context.Context) (storagew.Storage, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcsStorage{client: client}, nil
}

func (g *gcsStorage) Upload(ctx context.Context, bucket, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	bkt := g.client.Bucket(bucket)
	obj := bkt.Object(objectName)

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	// GCS handles chunking and resumability natively behind the scenes
	writer.ChunkSize = 10 * 1024 * 1024 // 10MB chunking enabled

	if _, err := io.Copy(writer, reader); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}

func (g *gcsStorage) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	return g.client.Bucket(bucket).Object(objectName).NewReader(ctx)
}

func (g *gcsStorage) Delete(ctx context.Context, bucket, objectName string) error {
	return g.client.Bucket(bucket).Object(objectName).Delete(ctx)
}

func (g *gcsStorage) GetPresignedURL(ctx context.Context, bucket, objectName string, expiry time.Duration) (string, error) {
	opts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}
	return g.client.Bucket(bucket).SignedURL(objectName, opts)
}

// --- Explicit Multipart Upload Operations ---
// GCS does not use the S3-style manual Init/UploadPart/Complete pattern.
// These methods safely return ErrNotSupported to prevent misconfiguration.

func (g *gcsStorage) InitMultipartUpload(ctx context.Context, bucket, objectName, contentType string) (string, error) {
	return "", storagew.ErrNotSupported
}

func (g *gcsStorage) UploadPart(ctx context.Context, bucket, objectName, uploadID string, partNumber int, reader io.Reader, partSize int64) (storagew.MultipartPart, error) {
	return storagew.MultipartPart{}, storagew.ErrNotSupported
}

func (g *gcsStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectName, uploadID string, parts []storagew.MultipartPart) error {
	return storagew.ErrNotSupported
}

func (g *gcsStorage) AbortMultipartUpload(ctx context.Context, bucket, objectName, uploadID string) error {
	return storagew.ErrNotSupported
}
