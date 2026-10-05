package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AddPhotoUpload records an image that has been stored and is in no post yet.
func (h *mediumHandler) AddPhotoUpload(ctx context.Context, req *graphpb.PhotoUploadAddRequest) (*graphpb.PhotoUploadAddResponse, error) {
	upload := req.GetUpload()
	if upload == nil || upload.OwnerId <= 0 || upload.CreatedBy <= 0 || upload.Filename == "" {
		return nil, status.Error(codes.InvalidArgument, "an upload needs an owner, an uploader and a filename")
	}

	id, err := h.env.Repos.PhotoFiles.AddUpload(&entities.PhotoUpload{
		OwnerID:   upload.OwnerId,
		CreatedBy: upload.CreatedBy,
		Filename:  upload.Filename,
		Mimetype:  upload.Mimetype,
		Width:     int(upload.Width),
		Height:    int(upload.Height),
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.PhotoUploadAddResponse{Id: id}, nil
}

// ReadPhotoUploads returns the uploads with these ids that still exist.
func (h *mediumHandler) ReadPhotoUploads(ctx context.Context, req *graphpb.PhotoUploadsReadRequest) (*graphpb.PhotoUploadsReadResponse, error) {
	uploads, err := h.env.Repos.PhotoFiles.ReadUploads(req.GetIds())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp := &graphpb.PhotoUploadsReadResponse{}
	for _, upload := range uploads {
		resp.Uploads = append(resp.Uploads, &graphpb.PhotoUpload{
			Id:        upload.ID,
			OwnerId:   upload.OwnerID,
			CreatedBy: upload.CreatedBy,
			Filename:  upload.Filename,
			Mimetype:  upload.Mimetype,
			Width:     int32(upload.Width),
			Height:    int32(upload.Height),
		})
	}

	return resp, nil
}

// BrowsePhotoFiles returns the images of these posts, each post's in order.
//
// No access check of its own. It is asked about posts the caller has already
// read, and reading is where access is decided.
func (h *mediumHandler) BrowsePhotoFiles(ctx context.Context, req *graphpb.PhotoFilesBrowseRequest) (*graphpb.PhotoFilesBrowseResponse, error) {
	files, err := h.env.Repos.PhotoFiles.Browse(req.GetNodeIds())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.PhotoFilesBrowseResponse{Files: photoFilesToGrpc(files)}, nil
}

// SetPhotoFiles makes these the images of a post, in the order given.
func (h *mediumHandler) SetPhotoFiles(ctx context.Context, req *graphpb.PhotoFilesSetRequest) (*graphpb.PhotoFilesSetResponse, error) {
	if req.GetNodeId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a post is required")
	}

	var files []*entities.PhotoFile
	for _, file := range req.GetFiles() {
		if file.Filename == "" {
			return nil, status.Error(codes.InvalidArgument, "an image needs a filename")
		}
		files = append(files, &entities.PhotoFile{
			Filename: file.Filename,
			Mimetype: file.Mimetype,
			Width:    int(file.Width),
			Height:   int(file.Height),
			AltText:  file.AltText,
		})
	}

	if err := h.env.Repos.PhotoFiles.Set(req.NodeId, files, req.GetUploadIds()); err != nil {
		log.Println(err)
		return nil, err
	}

	stored, err := h.env.Repos.PhotoFiles.Browse([]int64{req.NodeId})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.PhotoFilesSetResponse{Files: photoFilesToGrpc(stored)}, nil
}

func photoFilesToGrpc(files []*entities.PhotoFile) []*graphpb.PhotoFile {
	var out []*graphpb.PhotoFile
	for _, file := range files {
		out = append(out, &graphpb.PhotoFile{
			NodeId:   file.NodeID,
			Position: int32(file.Position),
			Filename: file.Filename,
			Mimetype: file.Mimetype,
			Width:    int32(file.Width),
			Height:   int32(file.Height),
			AltText:  file.AltText,
		})
	}
	return out
}
