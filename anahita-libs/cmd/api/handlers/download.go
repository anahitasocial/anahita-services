package handlers

import (
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Downloading what a post holds: a photo, a video.
//
// Whoever posted it says whether it may be downloaded, on the post, with
// what their profile starts new posts with as the default. See
// permissions.CanDownload for who that lets download, and the services'
// docs/photos.md and docs/videos.md.

// DownloadSigner signs the address a file is saved from.
type DownloadSigner interface {
	PresignDownload(path, name string) (string, error)
}

// As many downloads as one person may ask for in a minute. Generous: it is
// there to stop a script, not somebody saving an album.
var downloadLimit = ratelimit.Limit{Max: 60, Window: time.Minute}

// DefaultAllowDownload is what a new post starts with when its author did
// not say: what the profile it is posted on says. A group's choice for
// what is posted in the group, a person's own for their own profile.
func DefaultAllowDownload(said *bool, owner *entities.AnahitaNode) bool {
	if said != nil {
		return *said
	}
	return owner != nil && owner.AllowDownload
}

// MayDownload answers a request to download from a post: 0 when it may go
// ahead, or the status to refuse it with.
//
// Whoever cannot see the post is told it is not there, as for the post
// itself. Whoever can see it and may not download it is told so.
func MayDownload(filter VisibilityFilter, limiter ratelimit.Counter, post *entities.AnahitaNode, viewer *entities.Viewer) int {
	if post == nil || !permissions.HoldsAFile(post) || !MaySee(filter, post.ID, viewer) {
		return http.StatusNotFound
	}
	if !permissions.CanDownload(viewer, post) {
		return http.StatusForbidden
	}
	if limiter != nil && viewer != nil && viewer.ID > 0 &&
		ratelimit.Exceeded(limiter, "download", fmt.Sprint(viewer.ID), downloadLimit) {
		return http.StatusTooManyRequests
	}
	return 0
}

// DownloadName is what a downloaded file is called: the post's title, or
// the kind of thing and its number, with a part for which file of several
// and the ending of the file it is.
func DownloadName(post *entities.AnahitaNode, kind, part, key string) string {
	base := plainName(post.Name)
	if base == "" {
		base = fmt.Sprintf("%s-%d", kind, post.ID)
	}
	if part != "" {
		base += "-" + part
	}
	return base + path.Ext(key)
}

// SendDownload sends whoever asked on to the file, to be saved.
func SendDownload(c echo.Context, signer DownloadSigner, key, name string) error {
	address, err := signer.PresignDownload(key, name)
	if err != nil || address == "" {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Asked afresh each time: whether it may be downloaded can change.
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.Redirect(http.StatusFound, address)
}

// plainName is a title as part of a file's name: its letters and digits in
// lower case, with a dash for each run of anything else. Letters that are
// not in the plain alphabet are left out, and a title made only of those
// gives nothing, which the caller has a fallback for.
func plainName(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
		if b.Len() >= 80 {
			break
		}
	}
	return b.String()
}
