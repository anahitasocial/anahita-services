package storage

import (
	"mime/multipart"
)

type StorageInterface interface {
	Read(path string, public bool) ([]byte, error)
	Write(file *multipart.FileHeader, path string, data []byte, public bool) error
	Delete(path string, public bool) error
	Exists(path string, public bool) (bool, error)
	GetURL(path string, public bool) (string, error)
	Download(path string, public bool) ([]byte, error)
}

func NewStorage(config ConfigInterface) StorageInterface {
	// if config.StorageType == "aws-s3" {
	// 	return NewStorageAwsS3(config)
	// }

	return NewStorageAwsS3(config)
}
