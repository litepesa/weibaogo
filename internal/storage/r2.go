// ===============================
// internal/storage/r2.go - Cloudflare R2 Storage Client
// ===============================

package storage

import (
	"context"
	"fmt"
	"io"

	"weibaobe/internal/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type R2Client struct {
	client     *s3.S3
	bucketName string
	publicURL  string
}

func NewR2Client(cfg config.R2Config) (*R2Client, error) {
	// Create AWS session configured for R2
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 session: %w", err)
	}

	client := s3.New(sess)

	return &R2Client{
		client:     client,
		bucketName: cfg.BucketName,
		publicURL:  cfg.PublicURL,
	}, nil
}

func (r *R2Client) UploadFile(ctx context.Context, key string, file io.Reader, contentType string) error {
	_, err := r.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(key),
		Body:        aws.ReadSeekCloser(file),
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"), // Make files publicly readable
	})

	if err != nil {
		return fmt.Errorf("failed to upload file to R2: %w", err)
	}

	return nil
}

func (r *R2Client) DeleteFile(ctx context.Context, key string) error {
	_, err := r.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete file from R2: %w", err)
	}

	return nil
}

func (r *R2Client) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s", r.publicURL, key)
}

func (r *R2Client) FileExists(ctx context.Context, key string) (bool, error) {
	_, err := r.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		// Check if it's a "not found" error
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == "NotFound" {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}
