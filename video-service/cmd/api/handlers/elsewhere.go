package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"anahita-libs/permissions"
	"anahita-libs/ratelimit"
	libs_responses "anahita-libs/responses"
	"anahita-libs/videos"

	"video/data/repositories"

	"github.com/labstack/echo/v4"
)

// A video from another network, copied here the first time somebody plays
// it (roadmap row 120, docs/plans/audio-and-video.md). The web app asks for
// it when play is pressed, and asks again until it is ready; the worker
// does the copying (worker/elsewhere.go).

// How many videos from elsewhere one person may set copying in a while:
// each is a download and perhaps an encoding.
var elsewhereLimit = ratelimit.Limit{Max: 30, Window: 10 * time.Minute}

// ElsewhereRequest names the post whose video is asked for.
type ElsewhereRequest struct {
	ID int64 `param:"id" doc:"The post from another network"`
}

// ElsewhereStatus is how a video from elsewhere stands.
type ElsewhereStatus struct {
	// waiting or working while it is copied; ready, with the video to play;
	// failed, with why, when it is not to be had and the post is to lead to
	// its page instead.
	Status string `json:"status" doc:"waiting, working, ready or failed"`
	// Why it failed: too_large, too_long, not_a_video, gone, refused, or
	// failed for something here, which asking again tries again.
	Reason string `json:"reason,omitempty" doc:"Why it failed"`
	// When it is ready: what plays, as a video post's video.
	Video *libs_responses.Video `json:"video,omitempty" doc:"What plays, once ready"`
}

// AskElsewhere asks for a post's video from elsewhere to be copied here, or
// says it is played again.
//
// POST /videos/elsewhere/:id
func (h *videoHandler) AskElsewhere(c echo.Context) error {
	return h.elsewhere(c, true)
}

// ReadElsewhere says how a post's video from elsewhere stands.
//
// GET /videos/elsewhere/:id
func (h *videoHandler) ReadElsewhere(c echo.Context) error {
	return h.elsewhere(c, false)
}

func (h *videoHandler) elsewhere(c echo.Context, ask bool) error {
	viewer := getViewer(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}
	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, id, viewer) {
		return c.NoContent(http.StatusNotFound)
	}
	post, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil || post == nil || post.ID == 0 {
		return c.NoContent(http.StatusNotFound)
	}
	elsewhere := videos.ElsewhereFromMeta(post.Meta)
	if elsewhere == nil || elsewhere.Kind != videos.ElsewhereVideo || h.Elsewhere == nil {
		return c.NoContent(http.StatusNotFound)
	}

	// Copied already, and still kept: played again.
	if video := videos.FromMeta(post.Meta); video != nil {
		if ask {
			_, _ = h.Elsewhere.Ask(id, elsewhere.Kind, elsewhere.Source)
		}
		return c.JSON(http.StatusOK, &ElsewhereStatus{Status: repositories.ElsewhereReady, Video: libs_responses.NewVideo(post)})
	}

	if !ask {
		copied, err := h.Elsewhere.Read(id)
		if err != nil {
			log.Println("from elsewhere:", err)
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.JSON(http.StatusOK, statusOf(copied))
	}

	if h.Refused != nil && h.Refused(elsewhere.Host()) {
		return c.JSON(http.StatusOK, &ElsewhereStatus{Status: repositories.ElsewhereFailed, Reason: "refused"})
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "video_elsewhere", fmt.Sprint(viewer.ID), elsewhereLimit) {
		return c.JSON(http.StatusTooManyRequests, libs_responses.Refusal{Error: "too_many_videos"})
	}
	// Made once and no longer in the post (the post was changed where it
	// was written, which writes its meta again): made again.
	if before, err := h.Elsewhere.Read(id); err == nil && before != nil && before.Status == repositories.ElsewhereReady {
		_ = h.Elsewhere.Finish(id, repositories.ElsewhereRemoved, "")
	}
	copied, err := h.Elsewhere.Ask(id, elsewhere.Kind, elsewhere.Source)
	if err != nil {
		log.Println("from elsewhere:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, statusOf(copied))
}

// statusOf is how a copy stands to the web app: one that was ready and is
// gone from the post (removed, or the post changed) is asked for again.
func statusOf(copied *repositories.Copy) *ElsewhereStatus {
	if copied == nil {
		return &ElsewhereStatus{Status: repositories.ElsewhereRemoved}
	}
	switch copied.Status {
	case repositories.ElsewhereWaiting, repositories.ElsewhereWorking, repositories.ElsewhereFailed:
		return &ElsewhereStatus{Status: copied.Status, Reason: copied.Reason}
	}
	return &ElsewhereStatus{Status: repositories.ElsewhereRemoved}
}
