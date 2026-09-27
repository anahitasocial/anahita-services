package handlers

import (
	an_entities "anahita-libs/data/entities"
	"anahita-libs/data/services"
	images "anahita-libs/utils"
	"cover/constants"
	"cover/requests"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type coverHandler struct {
	*Env
}

// NewCoverHandler creates a new cover handler
func NewCoverHandler(env *Env) *coverHandler {
	return &coverHandler{
		env,
	}
}

// Add adds a new cover
func (h *coverHandler) Add(c echo.Context) error {
	var (
		err      error
		req      = requests.NewCoverRequest().Add
		fileReqA = services.NewFileRequest().Add
		viewer   = getViewer(c)
	)

	// Bind the request
	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Validate the request
	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Read the node (e.g., actor)
	node, err := h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Check if the user has permission to edit node
	if ok, err := h.canDo(node, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	file, err := c.FormFile("cover")
	if err != nil {
		log.Println("Error getting file:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	// Check if the file was uploaded
	if file == nil {
		log.Println("No file uploaded")
		return c.NoContent(http.StatusBadRequest)
	}

	// Open the file from the request
	fileHandler, err := file.Open()
	if err != nil {
		log.Println("Error opening file:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer fileHandler.Close()

	// Read the file content into a []byte
	fileBytes, err := io.ReadAll(fileHandler)
	if err != nil {
		log.Println("Error reading file:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Correct the orientation of the image
	fileBytes, err = images.CorrectOrientation(fileBytes)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Detect the MIME type of the corrected image
	mimeType := http.DetectContentType(fileBytes)
	if !images.IsValidMimeType(mimeType) {
		return c.NoContent(http.StatusUnsupportedMediaType)
	}

	// Check if the file size is valid
	if len(fileBytes) > constants.MaxCoverSize {
		return c.NoContent(http.StatusRequestEntityTooLarge)
	}

	// Generate a unique filename based on the MIME type
	node.CoverFilename, err = images.GenerateUniqueFilename(mimeType)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Retrieve cover sizes and process the image for each size
	covers, err := node.GetCovers()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, cover := range covers {
		// Resize the image for the current cover size
		content, err := images.ResizeImage(fileBytes, cover.Size.Width, cover.Size.Height)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		// Prepare the file request for uploading the cover
		fileReqA.Path = cover.Url
		fileReqA.Content = content
		fileReqA.Mimetype = mimeType
		fileReqA.Size = int64(len(content))

		// Upload the resized cover
		if _, err := h.Srv.FilesGRPC.Add(fileReqA); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// Update the node with the new cover
	if node.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(req.NodeID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		// Stamped as the viewer's edit. The graph refuses an edit with no
		// modifier, and an actor nobody has edited yet has none as read.
		actor.CoverFilename = node.CoverFilename
		actor.ModifiedBy = viewer.ID
		actor.UpdateTime = time.Now()
		if _, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	} else if node.IsMedium() {
		medium, err := h.Srv.MediaGRPC.Read(req.NodeID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		medium.CoverFilename = node.CoverFilename
		if _, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	services.PresignCover(h.Srv.FilesGRPC, node)

	return c.JSON(http.StatusCreated, node.Cover)
}

// Delete deletes an cover
func (h *coverHandler) Delete(c echo.Context) error {
	var (
		err     error
		req     = requests.NewCoverRequest().Delete
		fileReq = services.NewFileRequest().Delete
		viewer  = getViewer(c)
	)

	if err := c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err := c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.NodeID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Check if the user has permission to edit node
	if ok, err := h.canDo(node, viewer); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	covers, err := node.GetCovers()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, cover := range covers {
		fileReq.Path = cover.Url
		if err := h.Srv.FilesGRPC.Delete(fileReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// Update the node
	if node.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(req.NodeID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		actor.CoverFilename = ""
		actor.ModifiedBy = viewer.ID
		actor.UpdateTime = time.Now()
		if _, err = h.Srv.ActorsGRPC.Edit(actor, viewer); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	} else if node.IsMedium() {
		medium, err := h.Srv.MediaGRPC.Read(req.NodeID, viewer)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		medium.CoverFilename = ""
		if _, err = h.Srv.MediaGRPC.Edit(medium, viewer); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	return c.NoContent(http.StatusOK)
}

func (h *coverHandler) canDo(node *an_entities.AnahitaNode, viewer *an_entities.Viewer) (bool, error) {
	if node.IsActor() {
		actor, err := h.Srv.ActorsGRPC.Read(node.ID, viewer)
		if err != nil {
			return false, err
		}

		actorAccessCtx, err := h.getActorAccessContext(actor, viewer)
		if err != nil {
			return false, err
		}
		if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
			return false, err
		} else {
			return true, nil
		}
	} else if node.IsMedium() {
		medium, err := h.Srv.MediaGRPC.Read(node.ID, viewer)
		if err != nil {
			return false, err
		}

		mediumAccessCtx, err := h.getMediumAccessContext(medium, viewer)
		if err != nil {
			return false, err
		}
		if ok, err := h.Perms.Media.CanEdit(mediumAccessCtx); !ok {
			return false, err
		} else {
			return true, nil
		}
	}

	return false, nil
}
