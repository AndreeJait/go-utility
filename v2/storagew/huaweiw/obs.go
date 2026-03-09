package huaweiw

import (
	"context"
	"io"
	"time"

	"github.com/AndreeJait/go-utility/v2/storagew"
	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

// huaweiStorage implements the storagew.Storage interface for Huawei OBS.
type huaweiStorage struct {
	client *obs.ObsClient
}

// Config holds Huawei OBS credentials and endpoint mapping.
type Config struct {
	AccessKey string
	SecretKey string
	Endpoint  string
}

// New initializes a new Huawei OBS storage client.
func New(cfg *Config) (storagew.Storage, error) {
	client, err := obs.New(cfg.AccessKey, cfg.SecretKey, cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	return &huaweiStorage{client: client}, nil
}

func (h *huaweiStorage) Upload(ctx context.Context, bucket, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	input := &obs.PutObjectInput{}
	input.Bucket = bucket
	input.Key = objectName
	input.Body = reader
	input.ContentType = contentType

	_, err := h.client.PutObject(input)
	return err
}

func (h *huaweiStorage) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	input := &obs.GetObjectInput{}
	input.Bucket = bucket
	input.Key = objectName

	output, err := h.client.GetObject(input)
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (h *huaweiStorage) Delete(ctx context.Context, bucket, objectName string) error {
	input := &obs.DeleteObjectInput{}
	input.Bucket = bucket
	input.Key = objectName

	_, err := h.client.DeleteObject(input)
	return err
}

func (h *huaweiStorage) GetPresignedURL(ctx context.Context, bucket, objectName string, expiry time.Duration) (string, error) {
	input := &obs.CreateSignedUrlInput{}
	input.Method = obs.HttpMethodGet
	input.Bucket = bucket
	input.Key = objectName
	input.Expires = int(expiry.Seconds())

	output, err := h.client.CreateSignedUrl(input)
	if err != nil {
		return "", err
	}
	return output.SignedUrl, nil
}

func (h *huaweiStorage) InitMultipartUpload(ctx context.Context, bucket, objectName, contentType string) (string, error) {
	input := &obs.InitiateMultipartUploadInput{}
	input.Bucket = bucket
	input.Key = objectName
	input.ContentType = contentType

	output, err := h.client.InitiateMultipartUpload(input)
	if err != nil {
		return "", err
	}
	return output.UploadId, nil
}

func (h *huaweiStorage) UploadPart(ctx context.Context, bucket, objectName, uploadID string, partNumber int, reader io.Reader, partSize int64) (storagew.MultipartPart, error) {
	input := &obs.UploadPartInput{}
	input.Bucket = bucket
	input.Key = objectName
	input.UploadId = uploadID
	input.PartNumber = partNumber
	input.Body = reader
	input.PartSize = partSize

	output, err := h.client.UploadPart(input)
	if err != nil {
		return storagew.MultipartPart{}, err
	}
	return storagew.MultipartPart{
		PartNumber: output.PartNumber,
		ETag:       output.ETag,
	}, nil
}

func (h *huaweiStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectName, uploadID string, parts []storagew.MultipartPart) error {
	obsParts := make([]obs.Part, len(parts))
	for i, p := range parts {
		obsParts[i] = obs.Part{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	input := &obs.CompleteMultipartUploadInput{}
	input.Bucket = bucket
	input.Key = objectName
	input.UploadId = uploadID
	input.Parts = obsParts

	_, err := h.client.CompleteMultipartUpload(input)
	return err
}

func (h *huaweiStorage) AbortMultipartUpload(ctx context.Context, bucket, objectName, uploadID string) error {
	input := &obs.AbortMultipartUploadInput{}
	input.Bucket = bucket
	input.Key = objectName
	input.UploadId = uploadID

	_, err := h.client.AbortMultipartUpload(input)
	return err
}
