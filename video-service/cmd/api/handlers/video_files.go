package handlers

import (
	"log"
	"net/http"
	"strconv"

	"anahita-libs/cmd/api/handlers"
	"anahita-libs/data/services"
	"anahita-libs/videos"
	"video/constants"

	"github.com/labstack/echo/v4"
)

// File sends whoever asks to one of a video's files: a size of it,
// "720.mp4", or its poster, "poster.jpg".
//
// GET /videos/:id/files/:name
//
// A post says where its files are with addresses of this kind, which stay
// the same; each is answered with a redirect to a signed address in
// storage, good for a few minutes, where the file itself is read, in
// ranges, as a player asks for it. So a file is only ever reached by
// somebody who may see the post, asked each time, and never by an address
// that goes on working after the post is made private or removed.
func (h *videoHandler) File(c echo.Context) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}

	// Reading a post by its id asks nothing about who is reading. This does.
	if !handlers.MaySee(h.Srv.GraphNodeGRPC, id, viewer) {
		return c.NoContent(http.StatusNotFound)
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil || medium == nil || medium.Type != constants.NodeMediumVideoType {
		return c.NoContent(http.StatusNotFound)
	}

	video := videos.FromMeta(medium.Meta)
	if video == nil {
		return c.NoContent(http.StatusNotFound)
	}

	key := ""
	name := c.Param("name")
	if name == "poster.jpg" {
		key = video.Poster
	} else if height, ok := videos.FileHeight(name); ok {
		if rendition := video.Rendition(height); rendition != nil {
			key = rendition.Key
		}
	}
	if key == "" {
		return c.NoContent(http.StatusNotFound)
	}

	req := services.NewFileRequest().Presign
	req.Path = key
	address, err := h.Srv.FilesGRPC.Presign(req)
	if err != nil || address == "" {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Not kept by anything in between: who may see it is asked each time.
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.Redirect(http.StatusFound, address)
}
