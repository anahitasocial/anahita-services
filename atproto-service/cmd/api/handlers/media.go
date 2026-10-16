package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"atproto/bsky"

	libhandlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// The pictures of a post from elsewhere, copied here again.
//
// The pictures of a post that has been left alone for a while are removed
// from storage (anahita-libs/federation/media.go), and the post says so.
// Whoever wants to see them asks here, and they are read again from where
// the post came from and copied as they were the first time. Anybody
// signed in who may see the post may ask; it costs a fetch and some room,
// so it is not left without a limit.

// How often one person may ask for pictures to be copied again.
var restoreLimit = ratelimit.Limit{Max: 60, Window: 10 * time.Minute}

const (
	errMediaNotRemoved = "media_not_removed"
	errMediaGone       = "media_gone"
	errMediaFailed     = "media_failed"
	errMediaLimit      = "too_many_requests"
)

// RestorePictures copies the pictures of a post from Bluesky here again.
func (h *Handlers) RestorePictures(c echo.Context) error {
	viewer := getViewer(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || h.Keeper == nil {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "atproto_media", fmt.Sprint(viewer.ID), restoreLimit) {
		return refuse(c, http.StatusTooManyRequests, errMediaLimit)
	}

	post, err := h.Nodes.Read(id)
	if err != nil || post == nil || post.ID == 0 || post.Type != constants.NodeMediumPhotoEntityType {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}
	did, _, isPost := bsky.ParsePostURI(post.URI)
	if !isPost || federation.MediaRemovedFrom(post.Meta) == nil || !libhandlers.MaySee(h.Nodes, post.ID, viewer) {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*bsky.Timeout)
	defer cancel()

	restored, code := h.restorePictures(ctx, post, did)
	switch code {
	case "":
		return c.JSON(http.StatusOK, PicturesRestored{Restored: restored})
	case errMediaGone:
		return refuse(c, http.StatusGone, code)
	}
	return refuse(c, http.StatusBadGateway, code)
}

// restorePictures reads a post again from Bluesky and copies its pictures
// here. It answers with how many are stored now, or with why not.
func (h *Handlers) restorePictures(ctx context.Context, post *entities.AnahitaNode, did string) (int, string) {
	gone := func() (int, string) {
		// Taken away on Bluesky: the post says so from now on.
		if err := h.Keeper.PicturesGone(post); err != nil {
			log.Printf("saying that the pictures of %s are gone: %v", post.URI, err)
		}
		return 0, errMediaGone
	}

	record, err := h.Bluesky.PostAt(ctx, post.URI)
	if errors.Is(err, bsky.ErrNobody) {
		return gone()
	}
	if err != nil {
		log.Printf("the pictures of %s were not read again: %v", post.URI, err)
		return 0, errMediaFailed
	}

	var pictures []federation.Picture
	if !record.IsSensitive() {
		for _, each := range record.Pictures() {
			if len(pictures) == federation.PostPicturesMax {
				break
			}
			pictures = append(pictures, federation.Picture{URL: bsky.PictureURL(h.CDN, did, each.CID), Alt: each.Alt})
		}
	}
	restored, err := h.Keeper.RestorePictures(post, pictures)
	if err != nil {
		log.Printf("the pictures of %s were not copied again: %v", post.URI, err)
		return 0, errMediaFailed
	}
	if restored == 0 {
		return gone()
	}
	return restored, ""
}

// RunPictureSweep removes, now and then, the pictures that are due to go,
// until `ctx` is done. `sweep` does one round and says how many posts it
// was.
func RunPictureSweep(ctx context.Context, every time.Duration, sweep func() int) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if removed := sweep(); removed > 0 {
				log.Printf("the pictures of %d posts from Bluesky were removed from storage, to be copied again when asked for", removed)
			}
			timer.Reset(every)
		}
	}
}
