package services

import (
	"anahita-libs/proto/filepb"
	"bytes"
	"context"
	"file/config"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config" // Importing as awsConfig
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types" // Importing s3types for ObjectCannedACL
)

const (
	AwsS3AclPrivate = "private"
	AwsS3Encryption = "AES256"
)

type AwsS3ServiceInterface interface {
	Read(path string) (*filepb.File, error)
	Edit(file *filepb.File) error
	Add(file *filepb.File) error
	Delete(path string) error
	Exists(path string) (bool, error)
	Presign(path string) (string, error)
	BatchPresign(paths []string) ([]string, error)
}

func NewAwsS3Service(cfg *config.Config) *AwsS3Service {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO()) // Using awsConfig alias
	if err != nil {
		panic(err)
	}

	return &AwsS3Service{
		config:    cfg,
		awsConfig: awsCfg,
		acl:       s3types.ObjectCannedACLPrivate,
		encrypt:   s3types.ServerSideEncryptionAes256,
	}
}

type AwsS3Service struct {
	config    *config.Config
	awsConfig aws.Config
	acl       s3types.ObjectCannedACL
	encrypt   s3types.ServerSideEncryption
}

// Read reads a file from S3
func (s *AwsS3Service) Read(path string) (*filepb.File, error) {
	file := &filepb.File{}

	// Create a buffer to hold the downloaded content
	buffer := manager.NewWriteAtBuffer([]byte{})

	// Create a downloader directly from the config
	downloader := manager.NewDownloader(s3.NewFromConfig(s.awsConfig))

	objectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Aws.Bucket),
		Key:    aws.String(path),
	}

	_, err := downloader.Download(context.TODO(), buffer, objectInput)
	if err != nil {
		return nil, err
	}

	file.Content = buffer.Bytes()
	file.Size = int64(len(file.Content))
	file.Path = path
	file.Mimetype = "application/octet-stream"

	return file, nil
}

// Edit edits a file in S3
func (s *AwsS3Service) Edit(file *filepb.File) error {
	exists, err := s.Exists(file.Path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists %w", err)
	}
	if !exists {
		return fmt.Errorf("file does not exist")
	}

	client := s3.NewFromConfig(s.awsConfig)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:               aws.String(s.config.Aws.Bucket),
		Key:                  aws.String(file.Path),
		Body:                 bytes.NewReader(file.Content),
		ACL:                  s.acl,
		ContentType:          aws.String(file.Mimetype),
		ContentLength:        &file.Size,
		ServerSideEncryption: s.encrypt,
	})
	if err != nil {
		return err
	}

	return nil
}

// Add adds a file to S3
func (s *AwsS3Service) Add(file *filepb.File) error {
	exists, err := s.Exists(file.Path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists %w", err)
	}
	if exists {
		return fmt.Errorf("file already exists")
	}

	client := s3.NewFromConfig(s.awsConfig)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:               aws.String(s.config.Aws.Bucket),
		Key:                  aws.String(file.Path),
		Body:                 bytes.NewReader(file.Content),
		ACL:                  s.acl,
		ContentType:          aws.String(file.Mimetype),
		ContentLength:        &file.Size,
		ServerSideEncryption: s.encrypt,
	})
	if err != nil {
		return err
	}

	return nil
}

// Delete deletes a file from S3
func (s *AwsS3Service) Delete(path string) error {
	client := s3.NewFromConfig(s.awsConfig)

	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Aws.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return err
	}

	// Wait until the object is deleted
	waiter := s3.NewObjectNotExistsWaiter(client)
	err = waiter.Wait(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Aws.Bucket),
		Key:    aws.String(path),
	}, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to wait for object deletion: %w", err)
	}

	return nil
}

// Exists checks if a file exists in S3
func (s *AwsS3Service) Exists(path string) (bool, error) {
	client := s3.NewFromConfig(s.awsConfig)

	_, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Aws.Bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return false, nil
	}

	return true, nil
}

// Presign generates a presigned URL for a file in S3
func (s *AwsS3Service) Presign(path string) (string, error) {
	client := s3.NewFromConfig(s.awsConfig)

	presigner := s3.NewPresignClient(client)
	req, err := presigner.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.config.Aws.Bucket),
		Key:    aws.String(path),
	}, s3.WithPresignExpires(15*time.Minute))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

// BatchPresign generates presigned URLs for multiple files in S3
func (s *AwsS3Service) BatchPresign(paths []string) ([]string, error) {
	client := s3.NewFromConfig(s.awsConfig)
	presigner := s3.NewPresignClient(client)

	results := make([]string, len(paths))
	var wg sync.WaitGroup
	errs := make(chan error, len(paths))

	for i, path := range paths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			req, err := presigner.PresignGetObject(context.TODO(), &s3.GetObjectInput{
				Bucket: aws.String(s.config.Aws.Bucket),
				Key:    aws.String(path),
			}, s3.WithPresignExpires(15*time.Minute))
			if err != nil {
				errs <- err
				return
			}
			results[i] = req.URL // preserves order
		}(i, path)
	}

	wg.Wait()
	close(errs)
	if len(errs) > 0 {
		return nil, <-errs
	}

	return results, nil
}
