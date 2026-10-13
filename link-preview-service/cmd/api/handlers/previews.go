package handlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"anahita-libs/data/services"
	"anahita-libs/linkpreviews"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Previews of links: what a page somebody links to is called, what it is
// about, and its picture.
//
// A preview is asked for by whoever is writing a note, of the first link in
// it. The page is fetched once and kept a week, so a link many people post
// is read once. Its picture is copied, made small, into the installation's
// own storage: nobody reading a note is sent to another site for it.

type previewsHandler struct {
	*Env
}

// How many previews one person may ask for in a minute. Typing a link asks
// once; this is against using the service to fetch things.
var previewLimit = ratelimit.Limit{Max: 30, Window: time.Minute}

// Where a preview's picture is kept, and what its name looks like: the
// hash of what is in it, and what kind of picture it is.
const imageFolder = linkpreviews.StoragePrefix

var imageName = regexp.MustCompile(`^[a-f0-9]{64}\.(jpg|png|gif|webp)$`)

type previewRequest struct {
	URL string `query:"url" validate:"required,max=2000"`
}

// previewResponse is a preview as it is sent.
type previewResponse struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	// Where the picture is read from, on this installation.
	Image string `json:"image,omitempty"`
}

func respond(preview *linkpreviews.Preview) *previewResponse {
	snapshot := linkpreviews.NewSnapshot(preview)
	resp := &previewResponse{
		URL:         snapshot.URL,
		Title:       snapshot.Title,
		Description: snapshot.Description,
		SiteName:    snapshot.SiteName,
	}
	if snapshot.ImageKey != "" {
		resp.Image = linkpreviews.ImagePath + snapshot.ImageKey
	}

	return resp
}

// Read returns the preview of an address, fetching the page when no fresh
// preview is kept. 204 when there is nothing to show: the page could not be
// read, is not a page, or says nothing of itself.
func (h *previewsHandler) Read(c echo.Context) error {
	viewer := getViewer(c)
	if viewer == nil || !viewer.Authenticated || viewer.ID <= 0 {
		return c.NoContent(http.StatusForbidden)
	}

	req := &previewRequest{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	address, ok := linkpreviews.Normalise(req.URL)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	if ratelimit.Exceeded(h.Limiter, "link_preview", fmt.Sprint(viewer.ID), previewLimit) {
		return c.NoContent(http.StatusTooManyRequests)
	}

	preview, kept := h.Store.Get(address)
	if !kept {
		preview = h.fetch(address)
		if err := h.Store.Put(preview); err != nil {
			log.Println("link preview: could not keep the preview:", err)
		}
	}

	if !preview.Usable() {
		return c.NoContent(http.StatusNoContent)
	}

	return c.JSON(http.StatusOK, respond(preview))
}

// How many pages the other services may have read between them.
var internalLimit = ratelimit.Limit{Max: 120, Window: time.Minute}

// ReadInternal is Read for another service of this installation: the same
// preview, kept the same way, without somebody signed in to ask for it.
//
// It is asked by activitypub-service when a post arrives from another
// server with a link in it, so that the post has a card as one written
// here does: this installation reads the page itself and keeps its own
// copy of the picture, and no reader is sent to another site.
//
// It is not reachable from outside: the gateway routes /link-previews and
// this is not under it. It answers with the preview as it is kept, the
// key of its picture included, which Read does not give out.
func (h *previewsHandler) ReadInternal(c echo.Context) error {
	address, ok := linkpreviews.Normalise(c.QueryParam("url"))
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	preview, kept := h.Store.Get(address)
	if !kept {
		if ratelimit.Exceeded(h.Limiter, "link_preview_internal", "services", internalLimit) {
			return c.NoContent(http.StatusTooManyRequests)
		}
		preview = h.fetch(address)
		if err := h.Store.Put(preview); err != nil {
			log.Println("link preview: could not keep the preview:", err)
		}
	}

	if !preview.Usable() {
		return c.NoContent(http.StatusNoContent)
	}
	return c.JSON(http.StatusOK, preview)
}

// fetch reads a page and makes its preview. What went wrong, when
// something did, is logged without the address: a link somebody was about
// to post is theirs.
func (h *previewsHandler) fetch(address string) *linkpreviews.Preview {
	preview := &linkpreviews.Preview{URL: address, Status: linkpreviews.StatusFailed}

	page, final, err := h.Fetcher.Page(address)
	if err != nil {
		log.Println("link preview: page not read:", errorKind(err))
		return preview
	}

	preview.FinalURL = final
	preview.Title = linkpreviews.Clip(page.Title, linkpreviews.MaxTitle)
	preview.Description = linkpreviews.Clip(page.Description, linkpreviews.MaxText)
	preview.SiteName = linkpreviews.Clip(page.SiteName, linkpreviews.MaxSiteName)
	preview.Status = linkpreviews.StatusOK

	if page.Image != "" {
		preview.ImageKey = h.keepPicture(page.Image)
	}

	return preview
}

// keepPicture copies a page's picture into storage, made small, and returns
// the name it is kept under. "" when there is none worth keeping: a preview
// is shown without.
func (h *previewsHandler) keepPicture(address string) string {
	content, name, mimeType, err := h.Fetcher.Thumbnail(address)
	if err != nil {
		log.Println("link preview: picture not kept:", errorKind(err))
		return ""
	}

	// Named for what is in it: kept already is kept.
	exists := services.NewFileRequest().Exists
	exists.Path = imageFolder + name
	if found, err := h.Srv.FilesGRPC.Exists(exists); err == nil && found {
		return name
	}

	add := services.NewFileRequest().Add
	add.Path = imageFolder + name
	add.Content = content
	add.Mimetype = mimeType
	add.Size = int64(len(content))
	if _, err := h.Srv.FilesGRPC.Add(add); err != nil {
		log.Println("link preview: picture not stored:", err)
		return ""
	}

	return name
}

// errorKind is an error without what it was about. Go's network errors
// carry the address, and the address is not for the log.
func errorKind(err error) string {
	return fmt.Sprintf("%T", err)
}

// Image sends whoever asks to a preview's picture: a redirect to a signed
// address in storage, good for a short while.
//
// The name is the hash of the picture, which nobody can guess, and names
// nothing else: a name that is not that shape is not looked for.
func (h *previewsHandler) Image(c echo.Context) error {
	name := c.Param("name")
	if !imageName.MatchString(name) {
		return c.NoContent(http.StatusNotFound)
	}

	presign := services.NewFileRequest().Presign
	presign.Path = imageFolder + name

	address, err := h.Srv.FilesGRPC.Presign(presign)
	if err != nil || address == "" {
		return c.NoContent(http.StatusNotFound)
	}

	// Kept by the browser for less long than the signed address is good.
	c.Response().Header().Set("Cache-Control", "private, max-age=600")

	return c.Redirect(http.StatusFound, address)
}
