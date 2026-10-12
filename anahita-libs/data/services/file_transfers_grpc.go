package services

import (
	"context"

	"anahita-libs/proto/filepb"
)

// FileTransfersInterface is the part of the file service for large files:
// a video a browser sends straight to the store, in parts, and what a
// worker reads and writes there itself. See file-grpc-service's uploads.go
// for where each may go.
type FileTransfersInterface interface {
	// UploadStart begins an upload under uploads/, and returns its id.
	UploadStart(path string) (string, error)
	// UploadParts signs the addresses a browser writes these parts to.
	UploadParts(path, uploadID string, numbers []int32) ([]string, error)
	// UploadComplete makes one file of the parts sent and returns its
	// size. A file past maxSize is thrown away and refused.
	UploadComplete(path, uploadID string, maxSize int64) (int64, error)
	// UploadAbort throws an upload away.
	UploadAbort(path, uploadID string) error
	// Stat says whether a file is there, and how big it is.
	Stat(path string) (exists bool, size int64, err error)
	// FetchURL is an address to read a file at, for another service.
	FetchURL(path string) (string, error)
	// StoreURL is an address to write a worker's file to, as this type.
	StoreURL(path, contentType string) (string, error)
}

// NewFileTransfersGRPC is the file service's client, for large files.
func NewFileTransfersGRPC(config FileGRPCConfigInterface) FileTransfersInterface {
	return &FilesGRPC{config: config}
}

func (srv *FilesGRPC) UploadStart(path string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	res, err := client.UploadStart(context.Background(), &filepb.UploadStartRequest{Path: path})
	if err != nil {
		return "", err
	}
	return res.UploadId, nil
}

func (srv *FilesGRPC) UploadParts(path, uploadID string, numbers []int32) ([]string, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	res, err := client.UploadParts(context.Background(), &filepb.UploadPartsRequest{
		Path:        path,
		UploadId:    uploadID,
		PartNumbers: numbers,
	})
	if err != nil {
		return nil, err
	}
	return res.Urls, nil
}

func (srv *FilesGRPC) UploadComplete(path, uploadID string, maxSize int64) (int64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	res, err := client.UploadComplete(context.Background(), &filepb.UploadCompleteRequest{
		Path:     path,
		UploadId: uploadID,
		MaxSize:  maxSize,
	})
	if err != nil {
		return 0, err
	}
	return res.Size, nil
}

func (srv *FilesGRPC) UploadAbort(path, uploadID string) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.UploadAbort(context.Background(), &filepb.UploadAbortRequest{Path: path, UploadId: uploadID})
	return err
}

func (srv *FilesGRPC) Stat(path string) (bool, int64, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, 0, err
	}

	res, err := client.Stat(context.Background(), &filepb.StatRequest{Path: path})
	if err != nil {
		return false, 0, err
	}
	return res.Exists, res.Size, nil
}

func (srv *FilesGRPC) FetchURL(path string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	res, err := client.FetchURL(context.Background(), &filepb.TransferRequest{Path: path})
	if err != nil {
		return "", err
	}
	return res.Url, nil
}

func (srv *FilesGRPC) StoreURL(path, contentType string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	res, err := client.StoreURL(context.Background(), &filepb.TransferRequest{Path: path, ContentType: contentType})
	if err != nil {
		return "", err
	}
	return res.Url, nil
}
