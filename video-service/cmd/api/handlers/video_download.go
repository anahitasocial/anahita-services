package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"anahita-libs/videos"
	"log"
	"net/http"
	"strconv"
	"video/constants"

	"github.com/labstack/echo/v4"
)

// editDownloadRequest says whether a post may be downloaded.
type editDownloadRequest struct {
	ID    int64 `param:"id" validate:"required,numeric"`
	Allow bool  `json:"allow" form:"allow"`
}

// EditDownload says whether whoever can see a video may download it.
//
// PATCH /videos/:id/download
//
// A route of its own, as the post's audience has: an edit of its title
// must not change it. Whoever may edit the post may change this: its
// author, and the admins of the group it is on.
func (h *videoHandler) EditDownload(c echo.Context) error {
	var (
		req    = &editDownloadRequest{}
		viewer = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	medium, err := h.Srv.MediaGRPC.Read(req.ID, viewer)
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.mayEdit(c, medium, viewer, mediumAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	medium.AllowDownload = req.Allow
	medium.ModifiedBy = viewer.ID

	if medium, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, medium); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{medium})

	mediaResp := &responses.MediumNodeResponse{}

	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(medium))
}

// Download sends whoever asks to a video's file, to be saved: its largest
// size, or the one asked for.
//
// GET /videos/:id/download?size=<height>
//
// Only for somebody who can see the post, and only when its author has
// said it may be downloaded, or is who is asking. See
// permissions.CanDownload.
func (h *videoHandler) Download(c echo.Context) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		return c.NoContent(http.StatusNotFound)
	}
	if refused := handlers.MayDownload(h.Srv.GraphNodeGRPC, h.Limiter, medium, viewer); refused != 0 {
		return c.NoContent(refused)
	}

	video := videos.FromMeta(medium.Meta)
	if video == nil {
		return c.NoContent(http.StatusNotFound)
	}

	// Its sizes are kept largest first.
	rendition := &video.Renditions[0]
	if raw := c.QueryParam("size"); raw != "" {
		height, err := strconv.Atoi(raw)
		if err != nil {
			return c.NoContent(http.StatusNotFound)
		}
		if rendition = video.Rendition(height); rendition == nil {
			return c.NoContent(http.StatusNotFound)
		}
	}

	part := strconv.Itoa(rendition.Height) + "p"
	return handlers.SendDownload(c, h.Srv.Transfers, rendition.Key, handlers.DownloadName(medium, "video", part, rendition.Key))
}
