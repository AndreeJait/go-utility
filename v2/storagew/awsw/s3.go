// Package awsw provides a high-performance implementation of the storagew.Storage interface
// for Amazon S3, leveraging the 2026 AWS Transfer Manager v2.
package awsw

import (
	"context"
	"io"
	"time"

	"github.com/AndreeJait/go-utility/v2/storagew"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// awsStorage implements the storagew.Storage interface.
// It encapsulates the raw S3 Client and the new 2026 Transfer Manager Client
// to provide optimized object transfers and standard bucket operations.
type awsStorage struct {
	client     *s3.Client
	tm         *transfermanager.Client
	presignSvc *s3.PresignClient
}

// New initializes a new AWS S3 storage client using the 2026 Transfer Manager.
// This client addresses memory allocation issues (#2694) for non-seekable readers
// and provides a unified API for high-throughput object transfers.
func New(ctx context.Context, region string) (storagew.Storage, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	// Initialize the high-level Transfer Manager Client.
	// This replaces the legacy uploader/downloader with a single, highly optimized client.
	tm := transfermanager.New(client, func(o *transfermanager.Options) {
		o.PartSizeBytes = 10 * 1024 * 1024 // Default 10MB chunking
		o.Concurrency = 5                  // Number of parallel transfer threads
	})

	return &awsStorage{
		client:     client,
		tm:         tm,
		presignSvc: s3.NewPresignClient(client),
	}, nil
}

// Upload utilizes the new UploadObject API for memory-efficient streaming.
// It is specifically optimized for io.Reader sources that do not support seeking,
// such as multipart form-data streams from web frameworks.
func (a *awsStorage) Upload(ctx context.Context, bucket, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	_, err := a.tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectName),
		Body:        reader,
		ContentType: aws.String(contentType),
	})
	return err
}

// Download utilizes the Transfer Manager GetObject API which provides resilient,
// part-based downloads with an io.Reader interface. The returned io.ReadCloser
// is wrapped with a NopCloser as the Transfer Manager manages internal lifecycle.
func (a *awsStorage) Download(ctx context.Context, bucket, objectName string) (io.ReadCloser, error) {
	// The 2026 GetObject API within Transfer Manager returns a sequential io.Reader
	// while performing concurrent range-fetches in the background.
	res, err := a.tm.GetObject(ctx, &transfermanager.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, err
	}

	// We wrap the result in NopCloser to satisfy the io.ReadCloser interface
	// required by the storagew.Storage contract.
	return io.NopCloser(res.Body), nil
}

// Delete removes an object from the specified S3 bucket using the raw S3 client.
func (a *awsStorage) Delete(ctx context.Context, bucket, objectName string) error {
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectName),
	})
	return err
}

// GetPresignedURL generates a temporary URL that allows secure access to an object
// without requiring AWS credentials from the requester.
func (a *awsStorage) GetPresignedURL(ctx context.Context, bucket, objectName string, expiry time.Duration) (string, error) {
	req, err := a.presignSvc.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectName),
	}, s3.WithPresignExpires(expiry))

	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// --- Explicit Manual Multipart Upload Operations ---

// InitMultipartUpload initializes a manual multipart session.
// This is used for custom client-side chunking strategies.
func (a *awsStorage) InitMultipartUpload(ctx context.Context, bucket, objectName, contentType string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectName),
		ContentType: aws.String(contentType),
	}
	resp, err := a.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}
	return *resp.UploadId, nil
}

// UploadPart uploads a single chunk for an explicit manual multipart upload.
// The resulting MultipartPart containing the ETag must be stored to complete the upload later.
func (a *awsStorage) UploadPart(ctx context.Context, bucket, objectName, uploadID string, partNumber int, reader io.Reader, partSize int64) (storagew.MultipartPart, error) {
	input := &s3.UploadPartInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(objectName),
		UploadId:      aws.String(uploadID),
		PartNumber:    aws.Int32(int32(partNumber)),
		Body:          reader,
		ContentLength: aws.Int64(partSize),
	}
	resp, err := a.client.UploadPart(ctx, input)
	if err != nil {
		return storagew.MultipartPart{}, err
	}
	return storagew.MultipartPart{
		PartNumber: partNumber,
		ETag:       *resp.ETag,
	}, nil
}

// CompleteMultipartUpload assembles all uploaded chunks into the final object in S3.
// The parts slice must be provided in ascending order of PartNumber.
func (a *awsStorage) CompleteMultipartUpload(ctx context.Context, bucket, objectName, uploadID string, parts []storagew.MultipartPart) error {
	completedParts := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		completedParts[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(p.PartNumber)),
			ETag:       aws.String(p.ETag),
		}
	}

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(objectName),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	_, err := a.client.CompleteMultipartUpload(ctx, input)
	return err
}

// AbortMultipartUpload cancels a manual multipart upload session and removes
// any uploaded parts from the cloud to prevent storage costs.
func (a *awsStorage) AbortMultipartUpload(ctx context.Context, bucket, objectName, uploadID string) error {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(objectName),
		UploadId: aws.String(uploadID),
	}
	_, err := a.client.AbortMultipartUpload(ctx, input)
	return err
}
