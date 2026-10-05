package sweeper

import (
	"log"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"

	"graph/data/repositories"
)

const (
	// UploadHours is how long an uploaded image may wait to be put in a
	// post. Somebody who picked images and then closed the page has left
	// them stored; after this long they are removed.
	UploadHours = 24

	// uploadBatch is how many unused uploads one run removes.
	uploadBatch = 200
)

// Uploads removes the images that were uploaded for a photo post and never
// put in one, with every stored size of each. Returns how many it removed.
//
// The caller holds the sweeper's lock.
//
// The stored images go first and the record of the upload after, so a run
// that stops between the two leaves a record to find them by, not stored
// images nothing names.
func Uploads(repos *repositories.Repositories, files services.FilesGRPCInterface) int {
	cutoff := time.Now().UTC().Add(-UploadHours * time.Hour)

	uploads, err := repos.PhotoFiles.ExpiredUploads(cutoff, uploadBatch)
	if err != nil {
		log.Printf("sweeper: reading unused uploads failed: %v", err)
		return 0
	}
	if len(uploads) == 0 {
		return 0
	}

	var removed []int64
	for _, upload := range uploads {
		if files != nil {
			// Where a photo for this profile is stored.
			placeholder := &entities.AnahitaNode{}
			placeholder.Type = constants.NodeMediumPhotoEntityType
			placeholder.Service = constants.PhotoServiceName
			placeholder.OwnerID = upload.OwnerID

			portraits, err := placeholder.PortraitsFor(upload.Filename)
			if err != nil {
				log.Printf("sweeper: upload %d has no usable filename: %v", upload.ID, err)
			}
			for _, portrait := range portraits {
				if err := files.Delete(&services.FileDeleteRequest{Path: portrait.Url}); err != nil {
					log.Printf("sweeper: deleting %s failed: %v", portrait.Url, err)
				}
			}
		}
		removed = append(removed, upload.ID)
	}

	if err := repos.PhotoFiles.DeleteUploads(removed); err != nil {
		log.Printf("sweeper: forgetting %d unused upload(s) failed: %v", len(removed), err)
		return 0
	}

	log.Printf("sweeper: removed %d unused upload(s)", len(removed))
	return len(removed)
}
