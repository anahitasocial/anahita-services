package handlers

import (
	"anahita-libs/data/services"
	"anahita-libs/permissions"
	"anahita-libs/videos"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// File sends whoever asks to one of a video's files: a size of it,
// "720.mp4", or its poster, "poster.jpg"; or an audio post's, "audio.m4a"
// and "cover.jpg".
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
	if !permissions.CanReadByID(h.Srv.GraphNodeGRPC, id, viewer) {
		return c.NoContent(http.StatusNotFound)
	}

	medium, err := h.Srv.MediaGRPC.Read(id, viewer)
	// A video post, or a post from elsewhere whose video was copied here.
	if err != nil || medium == nil ||
		(!videos.IsOurs(medium.Type) && videos.ElsewhereFromMeta(medium.Meta) == nil) {
		return c.NoContent(http.StatusNotFound)
	}

	key := ""
	name := c.Param("name")
	// An audio post's one file and its cover (roadmap row 137).
	if audio := videos.AudioFromMeta(medium.Meta); audio != nil {
		switch name {
		case videos.AudioFile:
			key = audio.Key
		case videos.CoverFile:
			key = audio.Cover
		}
	} else if video := videos.FromMeta(medium.Meta); video != nil {
		if name == "poster.jpg" {
			key = video.Poster
		} else if height, ok := videos.FileHeight(name); ok {
			if rendition := video.Rendition(height); rendition != nil {
				key = rendition.Key
			}
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
