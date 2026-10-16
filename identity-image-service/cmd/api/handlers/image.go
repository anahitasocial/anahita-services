package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/responses"
	images "anahita-libs/utils"
	"errors"
	"identityimage/constants"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type imageHandler struct {
	*Env
}

// NewImageHandler creates the handler for avatars and covers.
func NewImageHandler(env *Env) *imageHandler {
	return &imageHandler{
		env,
	}
}

// SetAvatar gives an actor an avatar, in place of any it had.
func (h *imageHandler) SetAvatar(c echo.Context) error { return h.set(c, avatar) }

// RemoveAvatar takes an actor's avatar away.
func (h *imageHandler) RemoveAvatar(c echo.Context) error { return h.remove(c, avatar) }

// SetCover gives an actor or a post a cover, in place of any it had.
func (h *imageHandler) SetCover(c echo.Context) error { return h.set(c, cover) }

// RemoveCover takes a cover away.
func (h *imageHandler) RemoveCover(c echo.Context) error { return h.remove(c, cover) }

// Why a file was refused. Sent as "error", in the words the photo service
// uses for the same refusals, so a client handles them once.
const (
	errFileRequired    = "file_required"
	errFileUnsupported = "file_unsupported"
	errFileTooLarge    = "file_too_large"
	errFileUnreadable  = "file_unreadable"
)

func refuse(c echo.Context, status int, code string) error {
	return c.JSON(status, responses.Refusal{Error: code})
}

// refuseImage answers for a file images.Accept would not take.
func refuseImage(c echo.Context, err error) error {
	switch {
	case errors.Is(err, images.ErrUnsupported):
		return refuse(c, http.StatusUnsupportedMediaType, errFileUnsupported)
	case errors.Is(err, images.ErrTooLarge):
		return refuse(c, http.StatusRequestEntityTooLarge, errFileTooLarge)
	default:
		return refuse(c, http.StatusBadRequest, errFileUnreadable)
	}
}

func nodeID(c echo.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("node_id"), 10, 64)
	return id, err == nil && id > 0
}

// set stores a new image of this kind for a node and makes the node name it.
//
// In this order, each step leaving nothing half done behind it:
//
//  1. Every size of the new image is stored, under a name nothing uses yet.
//  2. The node is saved naming the new image. If that fails, what step 1
//     stored is removed again.
//  3. The image the node named before is removed from storage. Only now,
//     when nothing names it. This step was missing: every replaced avatar
//     and cover was left in storage for good.
func (h *imageHandler) set(c echo.Context, k kind) error {
	viewer := getViewer(c)

	id, ok := nodeID(c)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	node, status := h.editable(id, viewer, k)
	if node == nil {
		return c.NoContent(status)
	}

	header, err := c.FormFile(k.field)
	if err != nil || header == nil {
		return refuse(c, http.StatusBadRequest, errFileRequired)
	}

	opened, err := header.Open()
	if err != nil {
		log.Println(err)
		return refuse(c, http.StatusBadRequest, errFileUnreadable)
	}
	defer opened.Close()

	content, err := io.ReadAll(opened)
	if err != nil {
		log.Println(err)
		return refuse(c, http.StatusBadRequest, errFileUnreadable)
	}

	// The rules every uploaded image is held to; see images.Accept.
	content, mimeType, err := images.Accept(content, constants.MaxFileSize, constants.MaxPixels)
	if err != nil {
		log.Println(err)
		return refuseImage(c, err)
	}

	// What the node names now, to be removed once it names something else.
	var previous map[string]entities.AnahitaNodeImage
	if k.filename(node) != "" {
		previous, _ = k.sizes(node)
	}

	filename, err := images.GenerateUniqueFilename(mimeType)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	k.setFilename(node, filename)

	sizes, err := k.sizes(node)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// The picture brought down once to the widest size wanted. Every size is
	// made from that and not from what arrived, so a photo of tens of
	// megapixels is opened once, not once for each size.
	widest := 0
	for _, size := range sizes {
		widest = max(widest, size.Size.Width)
	}
	largest, err := images.FitImage(content, widest)
	if err != nil {
		log.Println(err)
		return refuse(c, http.StatusBadRequest, errFileUnreadable)
	}

	var stored []string
	for _, size := range sizes {
		var resized []byte

		if size.Size.Height > 0 && size.Size.Height == size.Size.Width {
			// The middle of the picture. This used to press the whole of it
			// into a square, so an avatar that was not square to begin with
			// came out squeezed.
			resized, err = images.SquareImage(largest, size.Size.Width)
		} else {
			resized, err = images.FitImage(largest, size.Size.Width)
		}
		if err != nil {
			log.Println(err)
			h.deleteStored(stored)
			return refuse(c, http.StatusBadRequest, errFileUnreadable)
		}
		if len(resized) > constants.MaxStoredSize {
			h.deleteStored(stored)
			return refuse(c, http.StatusRequestEntityTooLarge, errFileTooLarge)
		}

		fileReq := services.NewFileRequest().Add
		fileReq.Path = size.Url
		fileReq.Content = resized
		fileReq.Mimetype = mimeType
		fileReq.Size = int64(len(resized))
		if _, err := h.Srv.FilesGRPC.Add(fileReq); err != nil {
			log.Println(err)
			h.deleteStored(stored)
			return c.NoContent(http.StatusInternalServerError)
		}
		stored = append(stored, size.Url)
	}

	// Stamped as the viewer's edit, as every other edit is. The graph
	// refuses an edit with no modifier, and a person who has never edited
	// their profile has none on the node as read: every new member, on the
	// first thing onboarding asks of them.
	node.ModifiedBy = viewer.ID
	node.UpdatedAt = time.Now()

	if node.IsActor() {
		_, err = h.Srv.ActorsGRPC.Edit(node, viewer)
	} else {
		_, err = h.Srv.MediaGRPC.Edit(node, viewer)
	}
	if err != nil {
		log.Println(err)
		h.deleteStored(stored)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.deleteStored(paths(previous))

	// As every other answer sends a picture: `url` and `size`, in each
	// size there is.
	return c.JSON(http.StatusCreated, responses.NewAvatar(k.presigned(h.Srv.FilesGRPC, node)))
}

// remove takes an image of this kind off a node.
//
// The node stops naming the image first, and the image is removed from
// storage after. The other way round, a failure between the two leaves a
// profile naming a picture that is gone. This way round it leaves a stored
// picture nothing names, which nobody sees.
//
// Saving through an ordinary edit cannot do the first step: an edit leaves
// an image alone unless it is given a new name, so handing it an empty one
// changed nothing. That is what this used to do, and the profile went on
// naming a picture that had just been deleted.
func (h *imageHandler) remove(c echo.Context, k kind) error {
	viewer := getViewer(c)

	id, ok := nodeID(c)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	node, status := h.editable(id, viewer, k)
	if node == nil {
		return c.NoContent(status)
	}

	// Nothing to take away is not a failure.
	if k.filename(node) == "" {
		return c.NoContent(http.StatusOK)
	}

	sizes, err := k.sizes(node)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if _, err := h.Srv.GraphNodeGRPC.ClearFile(id, k.name, viewer.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	h.deleteStored(paths(sizes))

	return c.NoContent(http.StatusOK)
}

func paths(sizes map[string]entities.AnahitaNodeImage) []string {
	var out []string
	for _, size := range sizes {
		if size.Url != "" {
			out = append(out, size.Url)
		}
	}
	return out
}

// deleteStored removes stored images. A failure is logged and no more:
// nothing names these any longer, so there is nothing to try again from, and
// the change the person asked for has been made.
func (h *imageHandler) deleteStored(stored []string) {
	for _, path := range stored {
		fileReq := services.NewFileRequest().Delete
		fileReq.Path = path
		if err := h.Srv.FilesGRPC.Delete(fileReq); err != nil {
			log.Printf("deleting %s failed: %v", path, err)
		}
	}
}
