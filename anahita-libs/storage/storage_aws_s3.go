package storage

import (
	"bytes"
	"mime/multipart"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type storageAwsS3 struct {
	config     ConfigInterface
	sess       *session.Session
	encryption string
}

func getSession(config ConfigInterface) (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region: aws.String(config.GetRegion()),
	})
}

func NewStorageAwsS3(config ConfigInterface) StorageInterface {
	session, err := getSession(config)
	if err != nil {
		panic(err)
	}

	return &storageAwsS3{
		config:     config,
		sess:       session,
		encryption: "AES256",
	}
}

func (s *storageAwsS3) Read(path string, public bool) ([]byte, error) {
	return nil, nil
}

func (s *storageAwsS3) Write(fileHeader *multipart.FileHeader, key string, data []byte, public bool) error {
	fileContent, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer fileContent.Close()

	var size int64 = fileHeader.Size
	buffer := make([]byte, size)
	fileContent.Read(buffer)

	var contentType string = fileHeader.Header.Get("Content-Type")
	var acl string = AwsS3AclPrivate

	if public {
		acl = AwsS3AclPublic
	}

	_, err = s3.New(s.sess).PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(s.config.GetBucket()),
		Key:                  aws.String(key),
		ACL:                  aws.String(acl),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(size),
		ContentType:          aws.String(contentType),
		ServerSideEncryption: aws.String(s.encryption),
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *storageAwsS3) Delete(path string, public bool) error {
	return nil
}

func (s *storageAwsS3) Exists(path string, public bool) (bool, error) {
	return false, nil
}

func (s *storageAwsS3) GetURL(path string, public bool) (string, error) {
	svc := s3.New(s.sess)
	params := &s3.GetObjectInput{
		Bucket: aws.String(s.config.GetBucket()),
	}
	req, _ := svc.GetObjectRequest(params)

	if public {
		req.HTTPRequest.URL.Scheme = "https"
		req.HTTPRequest.URL.Host = s.config.GetBucket() + ".s3.amazonaws.com"
	} else {
		req.HTTPRequest.URL.Scheme = "https"
		req.HTTPRequest.URL.Host = s.config.GetBucket() + ".s3-" + s.config.GetRegion() + ".amazonaws.com"
	}

	if public {
		return req.HTTPRequest.URL.String(), nil
	}

	urlStr, err := req.Presign(AwsS3PresignExpireTime)

	if err != nil {
		return "", err
	}

	return urlStr, nil
}

func (s *storageAwsS3) Download(path string, public bool) ([]byte, error) {
	return nil, nil
}
