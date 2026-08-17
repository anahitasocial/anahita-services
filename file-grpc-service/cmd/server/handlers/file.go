package handlers

import (
	"anahita-libs/proto/filepb"
	"context"
	"log"
)

type fileHandler struct {
	filepb.FileServiceServer
	env *Env
}

func NewFileHandler(env *Env) *fileHandler {
	return &fileHandler{env: env}
}

// Read reads a file
func (h *fileHandler) Read(ctx context.Context, req *filepb.ReadRequest) (*filepb.ReadResponse, error) {
	log.Printf("Received Read request: %v", req.Path)

	file, err := h.env.Services.AwsS3.Read(req.Path)
	if err != nil {
		return nil, err
	}

	return &filepb.ReadResponse{
		File: file,
	}, nil
}

// Edit edits a file
func (h *fileHandler) Edit(ctx context.Context, req *filepb.EditRequest) (*filepb.EditResponse, error) {
	log.Printf("Received Edit request: %v", req.File.Path)

	err := h.env.Services.AwsS3.Edit(req.File)
	if err != nil {
		return nil, err
	}

	return &filepb.EditResponse{}, nil
}

// Add adds a file
func (h *fileHandler) Add(ctx context.Context, req *filepb.AddRequest) (*filepb.AddResponse, error) {
	log.Printf("Received Add request: %v", req.File.Path)

	err := h.env.Services.AwsS3.Add(req.File)
	if err != nil {
		return nil, err
	}

	return &filepb.AddResponse{}, nil
}

// Delete deletes a file
func (h *fileHandler) Delete(ctx context.Context, req *filepb.DeleteRequest) (*filepb.DeleteResponse, error) {
	log.Printf("Received Delete request: %v", req.Path)

	err := h.env.Services.AwsS3.Delete(req.Path)
	if err != nil {
		return nil, err
	}

	return &filepb.DeleteResponse{}, nil
}

// Exists checks if a file exists
func (h *fileHandler) Exists(ctx context.Context, req *filepb.ExistsRequest) (*filepb.ExistsResponse, error) {
	log.Printf("Received Exists request: %v", req.Path)

	exists, err := h.env.Services.AwsS3.Exists(req.Path)
	if err != nil {
		return nil, err
	}

	return &filepb.ExistsResponse{
		Exists: exists,
	}, nil
}

// Presign generates a presigned URL
func (h *fileHandler) Presign(ctx context.Context, req *filepb.PresignRequest) (*filepb.PresignResponse, error) {
	log.Printf("Received Presign request: %v", req.Path)

	url, err := h.env.Services.AwsS3.Presign(req.Path)
	if err != nil {
		return nil, err
	}

	return &filepb.PresignResponse{
		Url: url,
	}, nil
}

// BatchPresign generates presigned URLs for multiple files
func (h *fileHandler) BatchPresign(ctx context.Context, req *filepb.BatchPresignRequest) (*filepb.BatchPresignResponse, error) {
	log.Printf("Received BatchPresign request: %v", req.Paths)

	urls, err := h.env.Services.AwsS3.BatchPresign(req.Paths)
	if err != nil {
		return nil, err
	}

	return &filepb.BatchPresignResponse{
		Urls: urls,
	}, nil
}

// Ping checks if the service is up
func (h *fileHandler) Ping(ctx context.Context, req *filepb.PingRequest) (*filepb.PingResponse, error) {
	log.Printf("Received Ping request: %v", req)

	return &filepb.PingResponse{
		Message: "pong",
	}, nil
}
