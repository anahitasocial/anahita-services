package handlers

import (
	"linkpreview/config"
	"linkpreview/data/services"
	"linkpreview/fetch"

	"anahita-libs/data/entities"
	"anahita-libs/linkpreviews"
	an_mw "anahita-libs/middleware"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// PageFetcher reads a page and its picture. fetch.Fetcher is the one that
// does; a test has another.
type PageFetcher interface {
	Page(address string) (*fetch.Page, string, error)
	Thumbnail(address string) ([]byte, string, string, error)
}

type Env struct {
	Config  *config.Config
	Srv     *services.Services
	Fetcher PageFetcher
	// Previews while they are fresh. The same store text-service reads a
	// preview from when a note is saved.
	Store   *linkpreviews.Store
	Limiter ratelimit.Counter
}

type Handlers struct {
	Previews *previewsHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Previews: &previewsHandler{env},
	}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}
