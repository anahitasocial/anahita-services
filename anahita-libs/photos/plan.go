// Package photos holds the rules for which images a photo post may have,
// for photo-service and for a thread's notes with pictures in text-service
// (roadmap row 143), so that both judge them the same way.
package photos

import (
	"unicode/utf8"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
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
	ErrFilesRequired   = "files_required"
	ErrTooManyFiles    = "too_many_files"
	ErrFileNotFound    = "file_not_found"
	ErrFileRepeated    = "file_repeated"
	ErrUploadNotYours  = "upload_not_yours"
	ErrAltTextTooLong  = "alt_text_too_long"
	ErrFileUnreadable  = "file_unreadable"
	ErrFileUnsupported = "file_unsupported"
	ErrFileTooLarge    = "file_too_large"
)

// Plan is what a post's images are to become.
type Plan struct {
	// The images, in order.
	Files []*entities.PhotoFile
	// The uploads among them, which stop being uploads once the post has
	// them.
	UploadIDs []int64
	// Images the post had and is to have no longer. Their stored copies are
	// to be removed once the change is saved.
	Removed []*entities.PhotoFile
}

// PlanFiles works out what a post's images are to become, or says why not.
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
func PlanFiles(
	existing []*entities.PhotoFile,
	uploads []*entities.PhotoUpload,
	wanted []FileRef,
	viewerID int64,
	ownerID int64,
) (*Plan, string) {
	if len(wanted) == 0 {
		return nil, ErrFilesRequired
	}
	if len(wanted) > constants.PhotoMaxFiles() {
		return nil, ErrTooManyFiles
	}

	has := map[string]*entities.PhotoFile{}
	for _, file := range existing {
		has[file.Filename] = file
	}

	uploaded := map[int64]*entities.PhotoUpload{}
	for _, upload := range uploads {
		uploaded[upload.ID] = upload
	}

	plan := &Plan{}
	kept := map[string]bool{}

	for _, ref := range wanted {
		if utf8.RuneCountInString(ref.AltText) > constants.PhotoAltTextMaxLength {
			return nil, ErrAltTextTooLong
		}

		var file *entities.PhotoFile

		switch {
		case ref.UploadID > 0:
			upload, found := uploaded[ref.UploadID]
			if !found {
				return nil, ErrFileNotFound
			}
			if upload.CreatedBy != viewerID || upload.OwnerID != ownerID {
				return nil, ErrUploadNotYours
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
				return nil, ErrFileNotFound
			}
			copied := *current
			file = &copied

		default:
			return nil, ErrFileNotFound
		}

		if kept[file.Filename] {
			return nil, ErrFileRepeated
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

// RefusalStatus is the HTTP status a refusal is sent with.
func RefusalStatus(code string) int {
	switch code {
	case ErrUploadNotYours:
		return 403
	case ErrFileNotFound:
		return 404
	case ErrFileUnsupported:
		return 415
	case ErrFileTooLarge:
		return 413
	case ErrFileUnreadable:
		return 400
	default:
		return 422
	}
}

// FilesRefusal says why a post's images were refused, and how many a post
// may have.
type FilesRefusal struct {
	// Why: `files_required`, `too_many_files`, `file_not_found`,
	// `file_repeated`, `upload_not_yours`, `alt_text_too_long`,
	// `file_unreadable`, `file_unsupported` or `file_too_large`.
	Error string `json:"error"`
	// How many images a post may have.
	Max int `json:"max"`
}

// Refusal is the answer for a refusal: its status and its body.
func Refusal(code string) (int, FilesRefusal) {
	return RefusalStatus(code), FilesRefusal{Error: code, Max: constants.PhotoMaxFiles()}
}

// UploadIDs are the uploads a list of images names.
func UploadIDs(refs []FileRef) []int64 {
	var ids []int64
	for _, ref := range refs {
		if ref.UploadID > 0 {
			ids = append(ids, ref.UploadID)
		}
	}
	return ids
}
