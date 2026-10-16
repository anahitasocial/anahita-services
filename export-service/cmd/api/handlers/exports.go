// Package handlers answers for a person's copy of their own data: asking
// for one, seeing how it is getting on, fetching it and giving it up.
package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	an_mw "anahita-libs/middleware"
	libs_responses "anahita-libs/responses"
	"anahita-libs/sessions"

	"export/config"
	"export/data/repositories"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Env struct {
	Config    *config.Config
	Exports   repositories.ExportsInterface
	Transfers services.FileTransfersInterface
	Files     services.FilesGRPCInterface
	// Where auth-service records that somebody has just proved who they
	// are. nil means that cannot be checked, and asking is refused.
	Session sessions.KeyValueInterface
}

type ExportsHandler struct {
	*Env
}

func NewExportsHandler(env *Env) *ExportsHandler {
	return &ExportsHandler{env}
}

// ExportItem is an export as its owner is told of it.
type ExportItem struct {
	// Its id: a UUID.
	ID string `json:"id"`
	// `pending`, `building`, `ready`, `failed`, `cancelled` or `expired`.
	Status string `json:"status"`
	// How big the archive is, once it is ready.
	SizeBytes int64 `json:"size_bytes,omitempty"`
	// Why it failed, when it did.
	Error string `json:"error,omitempty"`
	// When it was asked for.
	CreatedAt time.Time `json:"created_at"`
	// When it was ready.
	ReadyAt *time.Time `json:"ready_at,omitempty"`
	// When the archive will be deleted.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Where the archive is fetched from, while it is ready: an address
	// good for a few minutes, made afresh each time this is read.
	DownloadURL string `json:"download_url,omitempty"`
}

// ExportList is the viewer's exports, and when they may ask again.
type ExportList struct {
	// The latest first.
	Data []*ExportItem `json:"data"`
	// How many days must pass between two exports. 0 for no wait.
	EveryDays int `json:"every_days"`
	// How many days an archive is kept once it is ready.
	KeepDays int `json:"keep_days"`
	// When they may next ask, while they must wait.
	NextAt *time.Time `json:"next_at,omitempty"`
}

// TooSoon refuses an export asked for before it is time, and says when.
type TooSoon struct {
	// Always `too_soon`.
	Error string `json:"error"`
	// When they may ask.
	NextAt time.Time `json:"next_at"`
}

// ExportRequest names one of the viewer's exports. The handler reads it
// with c.Param; this describes it, for the API reference.
type ExportRequest struct {
	// The export's id: a UUID.
	ID string `param:"id"`
}

func (h *ExportsHandler) respond(export *repositories.Export) *ExportItem {
	resp := &ExportItem{
		ID:        export.ID,
		Status:    export.Status,
		SizeBytes: export.SizeBytes,
		Error:     export.Error,
		CreatedAt: export.CreatedAt,
		ReadyAt:   export.ReadyAt,
		ExpiresAt: export.ExpiresAt,
	}

	if export.Status == repositories.StatusReady && export.PackageKey != "" {
		name := "my-data-" + export.CreatedAt.Format("2006-01-02") + ".zip"
		address, err := h.Transfers.PresignDownload(export.PackageKey, name)
		if err != nil {
			log.Println(err)
		}
		resp.DownloadURL = address
	}
	return resp
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

// nextAllowed is when this person may next ask, or the zero time when they
// may now.
func (h *ExportsHandler) nextAllowed(personID int64) (time.Time, error) {
	if h.Config.EveryDays <= 0 {
		return time.Time{}, nil
	}

	last, err := h.Exports.LastMade(personID)
	if err != nil || last.IsZero() {
		return time.Time{}, err
	}
	next := last.Add(time.Duration(h.Config.EveryDays) * 24 * time.Hour)
	if next.After(time.Now().UTC()) {
		return next, nil
	}
	return time.Time{}, nil
}

// Add asks for an export of the viewer's own data.
//
// POST /exports
//
// Somebody who has just proved who they are, and only them: an archive is
// everything a person wrote, including what only they may see, and a
// session left open on a shared computer must not be enough to take it.
// One at a time, and one a week unless the installation says otherwise.
func (h *ExportsHandler) Add(c echo.Context) error {
	viewer := getViewer(c)

	if h.Session == nil {
		log.Println("step-up: no session store; refusing")
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := sessions.RequireStepUp(h.Session, viewer.ID); err != nil {
		if errors.Is(err, sessions.ErrStepUpRequired) {
			return c.JSON(http.StatusForbidden, libs_responses.Refusal{Error: "step_up_required"})
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	next, err := h.nextAllowed(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !next.IsZero() {
		// Not counted against the proof: asking again when it is time
		// should not need another.
		return c.JSON(http.StatusTooManyRequests, &TooSoon{Error: "too_soon", NextAt: next})
	}

	userAgent := c.Request().UserAgent()
	if len(userAgent) > 255 {
		userAgent = userAgent[:255]
	}
	export := &repositories.Export{
		ID:        uuid.NewString(),
		PersonID:  viewer.ID,
		IP:        c.RealIP(),
		UserAgent: userAgent,
	}
	if err := h.Exports.Add(export); err != nil {
		if errors.Is(err, repositories.ErrActive) {
			return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "active"})
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// One proof, one export.
	if err := sessions.ClearStepUp(h.Session, viewer.ID); err != nil {
		log.Printf("step-up: clear failed (person=%d): %v", viewer.ID, err)
	}

	return c.JSON(http.StatusCreated, h.respond(export))
}

// Browse lists the viewer's exports, the latest first, and says when they
// may next ask for one.
//
// GET /exports
func (h *ExportsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)

	exports, err := h.Exports.OfPerson(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	data := make([]*ExportItem, 0, len(exports))
	for _, export := range exports {
		data = append(data, h.respond(export))
	}

	resp := &ExportList{
		Data:      data,
		EveryDays: h.Config.EveryDays,
		KeepDays:  h.Config.KeepDays,
	}
	if next, err := h.nextAllowed(viewer.ID); err == nil && !next.IsZero() {
		resp.NextAt = &next
	}
	return c.JSON(http.StatusOK, resp)
}

// own reads the export in the address, for whoever asked for it. Anybody
// else is told there is none.
func (h *ExportsHandler) own(c echo.Context) (*repositories.Export, int) {
	viewer := getViewer(c)

	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		return nil, http.StatusNotFound
	}
	export, err := h.Exports.Read(id)
	if err != nil {
		log.Println(err)
		return nil, http.StatusInternalServerError
	}
	if export == nil || export.PersonID != viewer.ID {
		return nil, http.StatusNotFound
	}
	return export, 0
}

// Read says how one export is getting on.
//
// GET /exports/:id
func (h *ExportsHandler) Read(c echo.Context) error {
	export, refused := h.own(c)
	if refused != 0 {
		return c.NoContent(refused)
	}
	return c.JSON(http.StatusOK, h.respond(export))
}

// Delete gives an export up while it is being made, or deletes its archive
// once it is.
//
// DELETE /exports/:id
func (h *ExportsHandler) Delete(c echo.Context) error {
	export, refused := h.own(c)
	if refused != 0 {
		return c.NoContent(refused)
	}

	if err := h.Exports.ToCancelled(export.ID); err != nil {
		if errors.Is(err, repositories.ErrNotInState) {
			return c.NoContent(http.StatusNoContent)
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if export.PackageKey != "" {
		req := services.NewFileRequest().Delete
		req.Path = export.PackageKey
		if err := h.Files.Delete(req); err != nil {
			log.Printf("could not remove %s: %v", export.PackageKey, err)
		}
	}
	return c.NoContent(http.StatusNoContent)
}
