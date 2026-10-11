package handlers

import (
	"recommendation/config"
	"recommendation/data/repositories"
	"recommendation/data/services"

	"anahita-libs/data/entities"
	an_mw "anahita-libs/middleware"

	"github.com/labstack/echo/v4"
)

type Env struct {
	Config *config.Config
	Repos  *repositories.Repositories
	Srv    *services.Services
}

type Handlers struct {
	Actors      *actorsHandler
	Preferences *preferencesHandler
	Dismissals  *dismissalsHandler
	Events      *eventsHandler
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{
		Actors:      &actorsHandler{env},
		Preferences: &preferencesHandler{env},
		Dismissals:  &dismissalsHandler{env},
		Events:      &eventsHandler{env},
	}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

// signedIn reports whether somebody is: suggestions are for a person, about
// that person, and nobody else is sent any.
func signedIn(viewer *entities.Viewer) bool {
	return viewer != nil && viewer.Authenticated && viewer.ID > 0
}
