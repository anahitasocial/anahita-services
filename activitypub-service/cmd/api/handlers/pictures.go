package handlers

import (
	"activitypub/ap"

	"anahita-libs/data/entities"
	"anahita-libs/federation"
)

// A post that arrives with pictures is kept as a photo post, its pictures
// copied here. What its author marked as sensitive is kept without them:
// there is nothing here yet that puts a picture behind a warning, and
// showing it plainly is not this installation's to decide.

// picturesToKeep is the pictures of a post that are kept, or none: for a
// post marked sensitive, or where there is nothing to keep pictures with.
func (h *Handlers) picturesToKeep(note *ap.ReceivedNote) []federation.Picture {
	if !h.keeper().CanKeepPictures() || note.Sensitive {
		return nil
	}
	var pictures []federation.Picture
	for _, each := range note.Pictures() {
		if len(pictures) == federation.PostPicturesMax {
			break
		}
		pictures = append(pictures, federation.Picture{URL: each.URL, Alt: each.Name})
	}
	return pictures
}

func (h *Handlers) copyPostPictures(post *entities.AnahitaNode, pictures []federation.Picture) {
	h.keeper().CopyPostPictures(post, pictures)
}

func (h *Handlers) deletePostPictures(was *entities.AnahitaNode, files []*entities.PhotoFile) {
	h.keeper().DeletePostPictures(was, files)
}
