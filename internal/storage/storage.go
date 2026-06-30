// Package storage provides media storage abstraction (local filesystem or S3-compatible).
package storage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// MediaStorage defines the interface for media storage operations.
type MediaStorage interface {
	Store(filename string, data []byte, contentType string) (string, error)
	Retrieve(path string) ([]byte, error)
	Delete(path string) error
}

// LocalStorage stores media on the local filesystem.
type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	os.MkdirAll(basePath, 0755)
	return &LocalStorage{basePath: basePath}
}

func (s *LocalStorage) Store(filename string, data []byte, contentType string) (string, error) {
	fullPath := filepath.Join(s.basePath, filename)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", err
	}

	return filename, nil
}

func (s *LocalStorage) Retrieve(path string) ([]byte, error) {
	fullPath := filepath.Join(s.basePath, path)
	return os.ReadFile(fullPath)
}

func (s *LocalStorage) Delete(path string) error {
	return os.Remove(filepath.Join(s.basePath, path))
}

// S3Storage stores media in S3-compatible storage.
type S3Storage struct {
	client   *s3.S3
	uploader *s3manager.Uploader
	bucket   string
	endpoint string
	prefix   string
}

func NewS3Storage(endpoint, bucket, region, accessKey, secretKey, prefix string) (*S3Storage, error) {
	cfg := &aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}

	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Storage{
		client:   s3.New(sess),
		uploader: s3manager.NewUploader(sess),
		bucket:   bucket,
		prefix:   prefix,
	}, nil
}

func (s *S3Storage) Store(filename string, data []byte, contentType string) (string, error) {
	key := s.prefix + filename

	_, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return key, nil
}

func (s *S3Storage) Retrieve(path string) ([]byte, error) {
	key := path
	if !strings.HasPrefix(path, s.prefix) {
		key = s.prefix + path
	}

	output, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

func (s *S3Storage) Delete(path string) error {
	key := path
	if !strings.HasPrefix(path, s.prefix) {
		key = s.prefix + path
	}

	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
