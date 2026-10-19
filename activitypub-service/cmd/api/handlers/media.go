package handlers

import (
	"anahita-libs/responses"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/fetch"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
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

// RestorePictures copies the pictures of a post from another server here
// again.
func (h *Handlers) RestorePictures(c echo.Context) error {
	viewer := getViewer(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "activitypub_media", fmt.Sprint(viewer.ID), restoreLimit) {
		return refuse(c, http.StatusTooManyRequests, errMediaLimit)
	}

	post, err := h.Nodes.Read(id)
	if err != nil || post == nil || post.ID == 0 || post.Type != constants.NodeMediumPhotoEntityType {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}
	if !strings.HasPrefix(post.URI, "https://") || federation.MediaRemovedFrom(post.Meta) == nil || !permissions.CanReadByID(h.Nodes, post.ID, viewer) {
		return refuse(c, http.StatusNotFound, errMediaNotRemoved)
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(post.URI)) {
		return refuse(c, http.StatusGone, errMediaGone)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*fetch.Timeout)
	defer cancel()

	keeper := h.keeper()
	gone := func() error {
		// Taken away on its own server: the post says so from now on.
		if err := keeper.PicturesGone(post); err != nil {
			log.Printf("saying that the pictures of %s are gone: %v", post.URI, err)
		}
		return refuse(c, http.StatusGone, errMediaGone)
	}

	var note ap.ReceivedNote
	_, err = h.Fetcher.Object(ctx, post.URI, &note)
	if errors.Is(err, fetch.ErrGone) {
		return gone()
	}
	if err != nil || note.ID == "" || !sameHost(note.ID, post.URI) {
		log.Printf("the pictures of %s were not read again: %v", post.URI, err)
		return refuse(c, http.StatusBadGateway, errMediaFailed)
	}

	restored, err := keeper.RestorePictures(post, h.picturesToKeep(&note))
	if err != nil {
		log.Printf("the pictures of %s were not copied again: %v", post.URI, err)
		return refuse(c, http.StatusBadGateway, errMediaFailed)
	}
	if restored == 0 {
		return gone()
	}
	return c.JSON(http.StatusOK, responses.PicturesRestored{Restored: restored})
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
				log.Printf("the pictures of %d posts from other servers were removed from storage, to be copied again when asked for", removed)
			}
			timer.Reset(every)
		}
	}
}
