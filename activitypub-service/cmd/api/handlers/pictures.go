package handlers

import (
	"context"
	"log"
	"time"

	"activitypub/ap"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	images "anahita-libs/utils"
)

// The pictures of a post from another server.
//
// A post that arrives with pictures is kept as a photo post, which is what
// a post with pictures is here: its words are the caption, and up to four
// pictures are its images, each with what its author said it shows. So it
// is drawn, opened and stepped through like any photo post, and is in its
// author's photos.
//
// The pictures are copied into this installation's storage, never shown
// from where they came from: a picture loaded straight from another server
// tells that server who looked at what. Each is checked to be a picture
// and nothing else and written out again at this installation's own sizes,
// as an upload is, so nothing a file carries besides its picture is kept.
//
// What its author marked as sensitive is kept without its pictures: there
// is nothing here yet that puts a picture behind a warning, and showing it
// plainly is not this installation's to decide.

const (
	// The most pictures of one post that are kept: what a photo post holds.
	postPicturesMax = 4
	// The limits an uploaded photo is held to (photo-service/constants).
	postPictureMaxBytes  = 10 << 20
	postPictureMaxPixels = 50_000_000
	postPictureMaxWidth  = 2560
	postPictureMaxStored = 4<<20 - 64<<10
)

// PhotoFiles is where a post's list of pictures is kept: the graph.
type PhotoFiles interface {
	BrowsePhotoFiles(nodeIDs []int64) ([]*entities.PhotoFile, error)
	SetPhotoFiles(nodeID int64, files []*entities.PhotoFile, uploadIDs []int64) ([]*entities.PhotoFile, error)
}

// picturesToKeep is the pictures of a post that are kept, or none: for a
// post marked sensitive, or where there is nothing to keep pictures with.
func (h *Handlers) picturesToKeep(note *ap.ReceivedNote) []ap.Attachment {
	if h.Pictures == nil || h.Files == nil || h.PhotoFiles == nil || note.Sensitive {
		return nil
	}
	pictures := note.Pictures()
	if len(pictures) > postPicturesMax {
		pictures = pictures[:postPicturesMax]
	}
	return pictures
}

// copyPostPictures fetches a post's pictures and keeps them as its images.
// Whatever goes wrong with one, the others are kept; with all of them, the
// post is left with its words.
func (h *Handlers) copyPostPictures(post *entities.AnahitaNode, pictures []ap.Attachment) {
	if post == nil || post.ID == 0 || post.Type != constants.NodeMediumPhotoEntityType || len(pictures) == 0 {
		return
	}
	// Kept already: a post is told of more than once, and its pictures do
	// not change.
	if have, err := h.PhotoFiles.BrowsePhotoFiles([]int64{post.ID}); err != nil || len(have) > 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var (
		files  []*entities.PhotoFile
		stored []string
	)
	for _, picture := range pictures {
		if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(picture.URL)) {
			continue
		}
		file, paths := h.copyPostPicture(ctx, post, picture)
		if file == nil {
			continue
		}
		files = append(files, file)
		stored = append(stored, paths...)
	}
	if len(files) == 0 {
		return
	}

	if _, err := h.PhotoFiles.SetPhotoFiles(post.ID, files, nil); err != nil {
		log.Printf("the pictures of %s were stored and not listed: %v", post.URI, err)
		h.deleteFiles(stored)
	}
}

// copyPostPicture keeps one picture at every size a photo is kept at, and
// answers with what to list it as and where it was stored.
func (h *Handlers) copyPostPicture(ctx context.Context, post *entities.AnahitaNode, picture ap.Attachment) (*entities.PhotoFile, []string) {
	raw, _, err := h.Pictures.Bytes(ctx, picture.URL, postPictureMaxBytes)
	if err != nil {
		log.Printf("a picture of %s was not fetched: %v", post.URI, err)
		return nil, nil
	}
	// Whatever the server said it was, it is looked at.
	content, mimeType, err := images.Accept(raw, postPictureMaxBytes, postPictureMaxPixels)
	if err != nil {
		log.Printf("a picture of %s is not one that is kept: %v", post.URI, err)
		return nil, nil
	}

	filename, err := images.GenerateUniqueFilename(mimeType)
	if err != nil {
		return nil, nil
	}
	sizes, err := post.PortraitsFor(filename)
	if err != nil {
		return nil, nil
	}

	largest, err := images.FitImage(content, postPictureMaxWidth)
	if err != nil {
		return nil, nil
	}
	width, height, err := images.Dimensions(largest)
	if err != nil {
		return nil, nil
	}

	var stored []string
	for name, size := range sizes {
		var resized []byte
		switch name {
		case entities.PortraitSizeSquare:
			resized, err = images.SquareImage(largest, size.Size.Width)
		case entities.PortraitSizeOriginal:
			resized = largest
		default:
			resized, err = images.FitImage(largest, size.Size.Width)
		}
		if err == nil && len(resized) > postPictureMaxStored {
			err = images.ErrTooLarge
		}
		if err == nil {
			add := services.NewFileRequest().Add
			add.Path, add.Content, add.Mimetype, add.Size = size.Url, resized, mimeType, int64(len(resized))
			_, err = h.Files.Add(add)
		}
		if err != nil {
			log.Printf("a picture of %s was not stored: %v", post.URI, err)
			h.deleteFiles(stored)
			return nil, nil
		}
		stored = append(stored, size.Url)
	}

	return &entities.PhotoFile{
		Filename: filename,
		Mimetype: mimeType,
		Width:    width,
		Height:   height,
		AltText:  cutRunes(picture.Name, 1500),
	}, stored
}

// deletePostPictures removes what is stored of a post's pictures, once the
// post itself is gone. `was` is the post as it was, and `files` its list.
func (h *Handlers) deletePostPictures(was *entities.AnahitaNode, files []*entities.PhotoFile) {
	var stored []string
	for _, file := range files {
		sizes, err := was.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		for _, size := range sizes {
			stored = append(stored, size.Url)
		}
	}
	h.deleteFiles(stored)
}
