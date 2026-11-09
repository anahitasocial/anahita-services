package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/photos"
	images "anahita-libs/utils"
	"errors"
)

// The rules for which images a post may have are anahita-libs/photos',
// shared with text-service for a thread's notes (roadmap row 143). These
// are their names here.

// FileRef names one image a post is to have.
type FileRef = photos.FileRef

// filePlan is what a post's images are to become.
type filePlan = photos.Plan

// Why a list of images was refused. Sent to the client as "error".
const (
	errFilesRequired   = photos.ErrFilesRequired
	errTooManyFiles    = photos.ErrTooManyFiles
	errFileNotFound    = photos.ErrFileNotFound
	errFileRepeated    = photos.ErrFileRepeated
	errUploadNotYours  = photos.ErrUploadNotYours
	errAltTextTooLong  = photos.ErrAltTextTooLong
	errFileUnreadable  = photos.ErrFileUnreadable
	errFileUnsupported = photos.ErrFileUnsupported
	errFileTooLarge    = photos.ErrFileTooLarge
)

// planFiles works out what a post's images are to become, or says why not
// (photos.PlanFiles).
func planFiles(
	existing []*entities.PhotoFile,
	uploads []*entities.PhotoUpload,
	wanted []FileRef,
	viewerID int64,
	ownerID int64,
) (*filePlan, string) {
	return photos.PlanFiles(existing, uploads, wanted, viewerID, ownerID)
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
	return photos.RefusalStatus(code)
}
