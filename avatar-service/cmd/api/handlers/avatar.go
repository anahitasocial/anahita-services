package handlers

import (
	"anahita-libs/data/services"
	an_services "anahita-libs/data/services"
	images "anahita-libs/utils"
	"avatar/constants"
	"avatar/requests"
	"io"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type avatarHandler struct {
	*Env
}

// NewAvatarHandler creates a new avatar handler
func NewAvatarHandler(env *Env) *avatarHandler {
	return &avatarHandler{
		env,
	}
}

// Add adds a new avatar
func (h *avatarHandler) Add(c echo.Context) error {
	var (
		err     error
		req     = requests.NewAvatarRequest().Add
		fileReq = services.NewFileRequest().Add
		viewer  = getViewer(c)
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
	node, err := h.Srv.ActorsGRPC.Read(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Check if the node is an actor
	if !node.IsActor() {
		log.Println("node is not an actor")
		return c.NoContent(http.StatusForbidden)
	}

	// Check if the user has permission to edit
	actorAccessCtx, err := h.getActorAccessContext(node, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	file, err := c.FormFile("avatar")
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
	if len(fileBytes) > constants.MaxAvatarSize {
		return c.NoContent(http.StatusRequestEntityTooLarge)
	}

	// Generate a unique filename based on the MIME type
	node.Filename, err = images.GenerateUniqueFilename(mimeType)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Retrieve avatar sizes and process the image for each size
	avatars, err := node.GetAvatars()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, avatar := range avatars {
		// Resize the image for the current avatar size
		content, err := images.ResizeImage(fileBytes, avatar.Size.Width, avatar.Size.Height)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}

		// Prepare the file request for uploading the avatar
		fileReq.Path = avatar.Url
		fileReq.Content = content
		fileReq.Mimetype = mimeType
		fileReq.Size = int64(len(content))

		// Upload the resized avatar
		if _, err := h.Srv.FilesGRPC.Add(fileReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	// Update the node with the new avatar
	if _, err = h.Srv.ActorsGRPC.Edit(node, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	an_services.PresignAvatar(h.Srv.FilesGRPC, node)

	return c.JSON(http.StatusCreated, node.Avatar)
}

// Delete deletes an avatar
func (h *avatarHandler) Delete(c echo.Context) error {
	var (
		err     error
		req     = requests.NewAvatarRequest().Delete
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

	node, err := h.Srv.ActorsGRPC.Read(req.NodeID, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if !node.IsActor() {
		log.Println("node is not an actor")
		return c.NoContent(http.StatusForbidden)
	}

	actorAccessCtx, err := h.getActorAccessContext(node, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if ok, err := h.Perms.Actors.CanEdit(actorAccessCtx); !ok {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	avatars, err := node.GetAvatars()
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, avatar := range avatars {
		fileReq.Path = avatar.Url
		if err := h.Srv.FilesGRPC.Delete(fileReq); err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	node.Filename = ""
	if _, err = h.Srv.ActorsGRPC.Edit(node, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
