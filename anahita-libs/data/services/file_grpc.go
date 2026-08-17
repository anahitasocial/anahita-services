package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/filepb"
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var filesGRPCConnectCounts int

type FileGRPCConfigInterface interface {
	GetFileServiceAddr() string
}

// FilesGRPCInterface is the interface for file service
type FilesGRPCInterface interface {
	// Read returns the file content
	Read(req *FileReadRequest) (*entities.File, error)

	// Edit updates a file
	Edit(req *FileEditRequest) (*entities.File, error)

	// Add creates a new file
	Add(req *FileAddRequest) (*entities.File, error)

	// Delete removes a file
	Delete(req *FileDeleteRequest) error

	// Exists checks if a file exists
	Exists(req *FileExistsRequest) (bool, error)

	// Presign returns a presigned url for the file
	Presign(req *FilePresignRequest) (string, error)

	// BatchPresign returns presigned urls for multiple files
	BatchPresign(paths []string) ([]string, error)
}

// NewFilesGRPC creates a new file service
func NewFilesGRPC(config FileGRPCConfigInterface) FilesGRPCInterface {
	return &FilesGRPC{
		config: config,
	}
}

// FilesGRPC is the file service
type FilesGRPC struct {
	config FileGRPCConfigInterface
}

// getClient returns the grpc client
func (srv *FilesGRPC) getClient() (filepb.FileServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	for {
		cc, err := grpc.NewClient(srv.config.GetFileServiceAddr(), opts...)

		if err != nil {
			log.Println("File GRPC not ready...")
			filesGRPCConnectCounts++
		} else {
			log.Println("Connected to File GRPC!")
			return filepb.NewFileServiceClient(cc), nil
		}

		if filesGRPCConnectCounts > 10 {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
		continue
	}
}

// Read returns the file content
func (srv *FilesGRPC) Read(req *FileReadRequest) (*entities.File, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	res, err := client.Read(context.Background(), &filepb.ReadRequest{
		Path: req.Path,
	})
	if err != nil {
		return nil, err
	}

	return &entities.File{
		Content:  res.File.Content,
		Path:     res.File.Path,
		Mimetype: res.File.Mimetype,
		Size:     res.File.Size,
	}, nil
}

// Edit updates a file
func (srv *FilesGRPC) Edit(req *FileEditRequest) (*entities.File, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	res, err := client.Edit(context.Background(), &filepb.EditRequest{
		File: &filepb.File{
			Content:  req.Content,
			Path:     req.Path,
			Mimetype: req.Mimetype,
			Size:     req.Size,
		},
	})
	if err != nil {
		return nil, err
	}

	return &entities.File{
		Content:  res.File.Content,
		Path:     res.File.Path,
		Mimetype: res.File.Mimetype,
		Size:     res.File.Size,
	}, nil
}

// Add creates a new file
func (srv *FilesGRPC) Add(req *FileAddRequest) (*entities.File, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	_, err = client.Add(context.Background(), &filepb.AddRequest{
		File: &filepb.File{
			Content:  req.Content,
			Path:     req.Path,
			Mimetype: req.Mimetype,
			Size:     req.Size,
		},
	})
	if err != nil {
		return nil, err
	}

	return &entities.File{
		Content:  req.Content,
		Path:     req.Path,
		Mimetype: req.Mimetype,
		Size:     req.Size,
	}, nil
}

// Delete removes a file
func (srv *FilesGRPC) Delete(req *FileDeleteRequest) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.Delete(context.Background(), &filepb.DeleteRequest{
		Path: req.Path,
	})
	if err != nil {
		return err
	}

	return nil
}

// Exists checks if a file exists
func (srv *FilesGRPC) Exists(req *FileExistsRequest) (bool, error) {
	client, err := srv.getClient()
	if err != nil {
		return false, err
	}

	res, err := client.Exists(context.Background(), &filepb.ExistsRequest{
		Path: req.Path,
	})
	if err != nil {
		return false, err
	}

	return res.Exists, nil
}

// Presign returns a presigned url for the file
func (srv *FilesGRPC) Presign(req *FilePresignRequest) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	res, err := client.Presign(context.Background(), &filepb.PresignRequest{
		Path: req.Path,
	})
	if err != nil {
		return "", err
	}

	return res.Url, nil
}

// BatchPresign returns presigned urls for multiple files
func (srv *FilesGRPC) BatchPresign(paths []string) ([]string, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	res, err := client.BatchPresign(context.Background(), &filepb.BatchPresignRequest{
		Paths: paths,
	})
	if err != nil {
		return nil, err
	}

	return res.Urls, nil
}
