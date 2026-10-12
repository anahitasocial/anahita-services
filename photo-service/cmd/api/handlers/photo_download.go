package handlers

import (
	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"log"
	"net/http"
	"photo/constants"
	"strconv"

	"github.com/labstack/echo/v4"
)

// editDownloadRequest says whether a post may be downloaded.
type editDownloadRequest struct {
	ID    int64 `param:"id" validate:"required,numeric"`
	Allow bool  `json:"allow" form:"allow"`
}

// EditDownload says whether whoever can see a photo may download it.
//
// PATCH /photos/:id/download
//
// A route of its own, as the post's audience has: an edit of its title
// must not change it. Whoever may edit the post may change this: its
// author, and the admins of the group it is on.
func (h *photoHandler) EditDownload(c echo.Context) error {
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
	if err != nil || medium == nil || medium.Type != constants.NodeMediumPhotoType {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
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

// Download sends whoever asks to one image of a photo post, to be saved:
// the largest copy kept.
//
// GET /photos/:id/download?file=<position>
//
// The first image when no position is given. One image at a time: a post
// holds at most a few, and each is asked for by itself.
//
// Only for somebody who can see the post, and only when its author has
// said it may be downloaded, or is who is asking. See
// permissions.CanDownload.
func (h *photoHandler) Download(c echo.Context) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}
	position := 0
	if raw := c.QueryParam("file"); raw != "" {
		if position, err = strconv.Atoi(raw); err != nil || position < 0 {
			return c.NoContent(http.StatusNotFound)
		}
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil || medium == nil || medium.Type != constants.NodeMediumPhotoType {
		return c.NoContent(http.StatusNotFound)
	}
	if refused := handlers.MayDownload(h.Srv.GraphNodeGRPC, h.Limiter, medium, viewer); refused != 0 {
		return c.NoContent(refused)
	}

	// Which image: the one at that place among the post's, or the one the
	// post itself names when it has no list.
	filename := ""
	files, err := h.Srv.MediaGRPC.BrowsePhotoFiles([]int64{medium.ID})
	if err != nil {
		log.Println(err)
	}
	for _, file := range files {
		if file.Position == position {
			filename = file.Filename
		}
	}
	if filename == "" && position == 0 {
		filename = medium.Filename
	}
	if filename == "" {
		return c.NoContent(http.StatusNotFound)
	}

	portraits, err := medium.PortraitsFor(filename)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	// The largest copy there is. An image stored before the largest size
	// was kept has only the smaller ones.
	key := ""
	for _, size := range []string{
		entities.PortraitSizeOriginal,
		entities.PortraitSizeXXLarge,
		entities.PortraitSizeXLarge,
		entities.PortraitSizeLarge,
	} {
		candidate := portraits[size].Url
		if candidate == "" {
			continue
		}
		if exists, _, err := h.Srv.Transfers.Stat(candidate); err == nil && exists {
			key = candidate
			break
		}
	}
	if key == "" {
		return c.NoContent(http.StatusNotFound)
	}

	part := ""
	if len(files) > 1 {
		part = strconv.Itoa(position + 1)
	}
	return handlers.SendDownload(c, h.Srv.Transfers, key, handlers.DownloadName(medium, "photo", part, key))
}
