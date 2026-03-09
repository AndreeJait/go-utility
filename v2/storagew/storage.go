package storagew

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrNotSupported is returned when a specific cloud provider does not support the requested operation.
	// For example, Google Cloud Storage (GCS) does not support manual explicit multipart uploads.
	ErrNotSupported = errors.New("storagew: operation not supported by this provider")
)

// MultipartPart represents a successfully uploaded chunk in a manual multipart upload process.
// Both PartNumber and ETag are strictly required by S3-compatible providers to finalize the upload.
type MultipartPart struct {
	PartNumber int
	ETag       string
}

// Storage defines the unified contract for all cloud storage providers.
// By strictly adhering to this interface, the application core remains entirely agnostic
// to the underlying cloud infrastructure (AWS, GCS, MinIO, or Huawei).
type Storage interface {
	// --- Standard Operations ---

	// Upload streams a file to the storage provider.
	// It automatically handles chunked/multipart uploads under the hood if the provider SDK supports it.
	// objectSize can be -1 if the size is unknown (though some providers may require it).
	Upload(ctx context.Context, bucket, objectName string, reader io.Reader, objectSize int64, contentType string) error

	// Download retrieves a file as a stream.
	// The caller is strictly responsible for calling Close() on the returned io.ReadCloser to prevent memory leaks.
	Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error)

	// Delete forcefully removes an object from the specified bucket.
	Delete(ctx context.Context, bucket, objectName string) error

	// GetPresignedURL generates a temporary, secure URL for direct client-side download or access.
	GetPresignedURL(ctx context.Context, bucket, objectName string, expiry time.Duration) (string, error)

	// --- Explicit Manual Multipart Upload Operations ---

	// InitMultipartUpload begins a manual multipart upload session and returns a unique UploadID.
	InitMultipartUpload(ctx context.Context, bucket, objectName, contentType string) (uploadID string, err error)

	// UploadPart uploads a single chunk of data using the UploadID. Part numbers must generally be between 1 and 10000.
	UploadPart(ctx context.Context, bucket, objectName, uploadID string, partNumber int, reader io.Reader, partSize int64) (MultipartPart, error)

	// CompleteMultipartUpload finishes the manual upload by commanding the cloud provider to stitch all the parts together.
	CompleteMultipartUpload(ctx context.Context, bucket, objectName, uploadID string, parts []MultipartPart) error

	// AbortMultipartUpload cancels the active upload session and deletes any orphaned parts from the cloud to free up space.
	AbortMultipartUpload(ctx context.Context, bucket, objectName, uploadID string) error
}
