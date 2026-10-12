package permissions

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/policy"
	"anahita-libs/videos"
)

// HoldsAFile says whether a post is of a kind with a file to download: a
// photo, a video.
func HoldsAFile(post *entities.AnahitaNode) bool {
	if post == nil {
		return false
	}
	return post.Type == constants.NodeMediumPhotoEntityType || post.Type == videos.NodeType
}

// CanDownload says whether the viewer may download what a post holds.
//
// It is asked of somebody who can already see the post: this adds nothing
// to who that is. Whoever posted it has said whether it may be downloaded
// (AllowDownload), which is no until they say yes; they can always have
// their own file back whatever they said.
//
// A visitor on an installation that shows visitors only the start of
// things is sent small pictures and no more, so no download either.
func CanDownload(viewer *entities.Viewer, post *entities.AnahitaNode) bool {
	if !HoldsAFile(post) || post.GetID() <= 0 || post.DeletedAt != nil {
		return false
	}

	signedIn := viewer != nil && viewer.ID > 0 && viewer.Authenticated
	if !signedIn && policy.PreviewOnly() {
		return false
	}

	if post.AllowDownload {
		return true
	}

	return signedIn && post.CreatedBy == viewer.ID
}
