package handlers

import (
	"context"

	"pulse/config"
	"pulse/data/services"
	"pulse/feeds"

	"anahita-libs/data/entities"
	"anahita-libs/domains"
	an_mw "anahita-libs/middleware"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// FeedReader finds and reads feeds. feeds.Reader is the one that does; a
// test has another.
type FeedReader interface {
	Find(ctx context.Context, address string) (*feeds.Found, error)
	Read(ctx context.Context, feedURL, etag, lastModified string) (*feeds.Read, error)
}

// DomainChecker looks for a domain's token. domains.Checker is the one that
// does.
type DomainChecker interface {
	Check(ctx context.Context, site domains.Site, token string) (string, error)
}

type Env struct {
	Config  *config.Config
	Srv     *services.Services
	Feeds   FeedReader
	Domains DomainChecker
	Limiter ratelimit.Counter
}

type Handlers struct {
	Connect *connectHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Connect: &connectHandler{env},
	}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}
