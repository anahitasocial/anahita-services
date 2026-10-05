package services

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"log"
)

// AddPhotoUpload records an image that has been stored and is in no post yet.
func (srv *MediaGRPC) AddPhotoUpload(upload *entities.PhotoUpload) (int64, error) {
	client, err := srv.getClient()
	if err != nil {
		return 0, err
	}

	resp, err := client.AddPhotoUpload(context.Background(), &graphpb.PhotoUploadAddRequest{
		Upload: &graphpb.PhotoUpload{
			OwnerId:   upload.OwnerID,
			CreatedBy: upload.CreatedBy,
			Filename:  upload.Filename,
			Mimetype:  upload.Mimetype,
			Width:     int32(upload.Width),
			Height:    int32(upload.Height),
		},
	})
	if err != nil {
		return 0, err
	}

	return resp.GetId(), nil
}

// ReadPhotoUploads returns the uploads with these ids that still exist.
func (srv *MediaGRPC) ReadPhotoUploads(ids []int64) ([]*entities.PhotoUpload, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ReadPhotoUploads(context.Background(), &graphpb.PhotoUploadsReadRequest{Ids: ids})
	if err != nil {
		return nil, err
	}

	var uploads []*entities.PhotoUpload
	for _, upload := range resp.GetUploads() {
		uploads = append(uploads, &entities.PhotoUpload{
			ID:        upload.Id,
			OwnerID:   upload.OwnerId,
			CreatedBy: upload.CreatedBy,
			Filename:  upload.Filename,
			Mimetype:  upload.Mimetype,
			Width:     int(upload.Width),
			Height:    int(upload.Height),
		})
	}

	return uploads, nil
}

// BrowsePhotoFiles returns the images of these photo posts.
func (srv *MediaGRPC) BrowsePhotoFiles(nodeIDs []int64) ([]*entities.PhotoFile, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}

	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.BrowsePhotoFiles(context.Background(), &graphpb.PhotoFilesBrowseRequest{NodeIds: nodeIDs})
	if err != nil {
		return nil, err
	}

	return grpcToPhotoFiles(resp.GetFiles()), nil
}

// SetPhotoFiles makes these the images of a post, in the order given.
func (srv *MediaGRPC) SetPhotoFiles(nodeID int64, files []*entities.PhotoFile, uploadIDs []int64) ([]*entities.PhotoFile, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	req := &graphpb.PhotoFilesSetRequest{NodeId: nodeID, UploadIds: uploadIDs}
	for _, file := range files {
		req.Files = append(req.Files, &graphpb.PhotoFile{
			Filename: file.Filename,
			Mimetype: file.Mimetype,
			Width:    int32(file.Width),
			Height:   int32(file.Height),
			AltText:  file.AltText,
		})
	}

	resp, err := client.SetPhotoFiles(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return grpcToPhotoFiles(resp.GetFiles()), nil
}

func grpcToPhotoFiles(files []*graphpb.PhotoFile) []*entities.PhotoFile {
	var out []*entities.PhotoFile
	for _, file := range files {
		out = append(out, &entities.PhotoFile{
			NodeID:   file.NodeId,
			Position: int(file.Position),
			Filename: file.Filename,
			Mimetype: file.Mimetype,
			Width:    int(file.Width),
			Height:   int(file.Height),
			AltText:  file.AltText,
		})
	}
	return out
}

// PhotoFilesBrowser is the one call AttachPhotoFiles needs.
type PhotoFilesBrowser interface {
	BrowsePhotoFiles(nodeIDs []int64) ([]*entities.PhotoFile, error)
}

// AttachPhotoFiles gives each photo post among these nodes its images, with
// one call for all of them. Everything that is not a photo is left alone.
//
// A failure is logged and nothing is attached: the post still has the one
// image its node names, which is how a photo looked before it could hold
// more.
func AttachPhotoFiles(media PhotoFilesBrowser, nodes []*entities.AnahitaNode) {
	byID := map[int64][]*entities.AnahitaNode{}
	var ids []int64
	for _, node := range nodes {
		if node == nil || node.Type != constants.NodeMediumPhotoEntityType {
			continue
		}
		if _, seen := byID[node.ID]; !seen {
			ids = append(ids, node.ID)
		}
		byID[node.ID] = append(byID[node.ID], node)
		node.Files = nil
	}

	if len(ids) == 0 {
		return
	}

	files, err := media.BrowsePhotoFiles(ids)
	if err != nil {
		log.Println("reading the images of photo posts failed:", err)
		return
	}

	for _, file := range files {
		for _, node := range byID[file.NodeID] {
			// A copy for each node, since each gets its own addresses.
			copied := *file
			node.Files = append(node.Files, &copied)
		}
	}

	for _, id := range ids {
		for _, node := range byID[id] {
			node.SetFilePortraits()
		}
	}
}

// BatchPresignPhotoFiles turns the stored paths of every image of these
// nodes into addresses a browser can load, with one call for all of them.
func BatchPresignPhotoFiles(srv FilesGRPCInterface, nodes []*entities.AnahitaNode) {
	type place struct {
		file *entities.PhotoFile
		size string
	}

	var paths []string
	var places []place
	for _, node := range nodes {
		if node == nil {
			continue
		}
		for _, file := range node.Files {
			for size, image := range file.Portrait {
				if image.Url == "" {
					continue
				}
				paths = append(paths, image.Url)
				places = append(places, place{file, size})
			}
		}
	}

	if len(paths) == 0 {
		return
	}

	presigned, err := srv.BatchPresign(paths)
	if err != nil {
		log.Println(err)
		return
	}

	for i, at := range places {
		if i >= len(presigned) {
			break
		}
		image := at.file.Portrait[at.size]
		image.Url = presigned[i]
		at.file.Portrait[at.size] = image
	}
}
