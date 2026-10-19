package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	images "anahita-libs/utils"
	"errors"
	"unicode/utf8"
)

// FileRef names one image a post is to have: one it has already, by its id,
// or one that was uploaded for it, by the upload's id.
type FileRef struct {
	// An image the post has already, by its id.
	ID string `json:"id"`
	// An image uploaded for it, by the upload's id.
	UploadID int64 `json:"upload_id"`
	// What it shows, for people who cannot see it.
	AltText string `json:"alt_text"`
}

// Why a list of images was refused. Sent to the client as "error".
const (
	errFilesRequired   = "files_required"
	errTooManyFiles    = "too_many_files"
	errFileNotFound    = "file_not_found"
	errFileRepeated    = "file_repeated"
	errUploadNotYours  = "upload_not_yours"
	errAltTextTooLong  = "alt_text_too_long"
	errFileUnreadable  = "file_unreadable"
	errFileUnsupported = "file_unsupported"
	errFileTooLarge    = "file_too_large"
)

// filePlan is what a post's images are to become.
type filePlan struct {
	// The images, in order.
	Files []*entities.PhotoFile
	// The uploads among them, which stop being uploads once the post has
	// them.
	UploadIDs []int64
	// Images the post had and is to have no longer. Their stored copies are
	// to be removed once the change is saved.
	Removed []*entities.PhotoFile
}

// planFiles works out what a post's images are to become, or says why not.
//
// existing is what the post has now, empty for a new post. uploads are the
// uploads the request names that still exist. Nothing is read or written
// here, so every rule about which images a post may have is in one place and
// can be tested without a server.
//
// The rules:
//   - at least one image, at most constants.PhotoMaxFiles();
//   - each is either one the post already has or an upload;
//   - an upload can be used only by whoever uploaded it, and only on the
//     profile it was uploaded for, because that is where it is stored;
//   - no image twice.
func planFiles(
	existing []*entities.PhotoFile,
	uploads []*entities.PhotoUpload,
	wanted []FileRef,
	viewerID int64,
	ownerID int64,
) (*filePlan, string) {
	if len(wanted) == 0 {
		return nil, errFilesRequired
	}
	if len(wanted) > constants.PhotoMaxFiles() {
		return nil, errTooManyFiles
	}

	has := map[string]*entities.PhotoFile{}
	for _, file := range existing {
		has[file.Filename] = file
	}

	uploaded := map[int64]*entities.PhotoUpload{}
	for _, upload := range uploads {
		uploaded[upload.ID] = upload
	}

	plan := &filePlan{}
	kept := map[string]bool{}

	for _, ref := range wanted {
		if utf8.RuneCountInString(ref.AltText) > constants.PhotoAltTextMaxLength {
			return nil, errAltTextTooLong
		}

		var file *entities.PhotoFile

		switch {
		case ref.UploadID > 0:
			upload, found := uploaded[ref.UploadID]
			if !found {
				return nil, errFileNotFound
			}
			if upload.CreatedBy != viewerID || upload.OwnerID != ownerID {
				return nil, errUploadNotYours
			}
			file = &entities.PhotoFile{
				Filename: upload.Filename,
				Mimetype: upload.Mimetype,
				Width:    upload.Width,
				Height:   upload.Height,
			}
			plan.UploadIDs = append(plan.UploadIDs, upload.ID)

		case ref.ID != "":
			current, found := has[ref.ID]
			if !found {
				return nil, errFileNotFound
			}
			copied := *current
			file = &copied

		default:
			return nil, errFileNotFound
		}

		if kept[file.Filename] {
			return nil, errFileRepeated
		}
		kept[file.Filename] = true

		file.AltText = ref.AltText
		file.Position = len(plan.Files)
		plan.Files = append(plan.Files, file)
	}

	for _, file := range existing {
		if !kept[file.Filename] {
			plan.Removed = append(plan.Removed, file)
		}
	}

	return plan, ""
}

// refusalFor is the code for why an uploaded file was not accepted as an
// image.
func refusalFor(err error) string {
	switch {
	case errors.Is(err, images.ErrUnsupported):
		return errFileUnsupported
	case errors.Is(err, images.ErrTooLarge):
		return errFileTooLarge
	default:
		return errFileUnreadable
	}
}

// refusalStatus is the HTTP status a refusal is sent with.
func refusalStatus(code string) int {
	switch code {
	case errUploadNotYours:
		return 403
	case errFileNotFound:
		return 404
	case errFileUnsupported:
		return 415
	case errFileTooLarge:
		return 413
	case errFileUnreadable:
		return 400
	default:
		return 422
	}
}
