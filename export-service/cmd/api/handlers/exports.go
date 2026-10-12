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

type exportsHandler struct {
	*Env
}

func NewExportsHandler(env *Env) *exportsHandler {
	return &exportsHandler{env}
}

// exportResponse is an export as its owner is told of it.
type exportResponse struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	SizeBytes int64      `json:"size_bytes,omitempty"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ReadyAt   *time.Time `json:"ready_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// Where the archive is fetched from, while it is ready: an address
	// good for a few minutes, made afresh each time this is read.
	DownloadURL string `json:"download_url,omitempty"`
}

func (h *exportsHandler) respond(export *repositories.Export) *exportResponse {
	resp := &exportResponse{
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
func (h *exportsHandler) nextAllowed(personID int64) (time.Time, error) {
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
func (h *exportsHandler) Add(c echo.Context) error {
	viewer := getViewer(c)

	if h.Session == nil {
		log.Println("step-up: no session store; refusing")
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := sessions.RequireStepUp(h.Session, viewer.ID); err != nil {
		if errors.Is(err, sessions.ErrStepUpRequired) {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "step_up_required"})
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
		return c.JSON(http.StatusTooManyRequests, map[string]any{"error": "too_soon", "next_at": next})
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
			return c.JSON(http.StatusConflict, map[string]string{"error": "active"})
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
func (h *exportsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)

	exports, err := h.Exports.OfPerson(viewer.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	data := make([]*exportResponse, 0, len(exports))
	for _, export := range exports {
		data = append(data, h.respond(export))
	}

	resp := map[string]any{
		"data":       data,
		"every_days": h.Config.EveryDays,
		"keep_days":  h.Config.KeepDays,
	}
	if next, err := h.nextAllowed(viewer.ID); err == nil && !next.IsZero() {
		resp["next_at"] = next
	}
	return c.JSON(http.StatusOK, resp)
}

// own reads the export in the address, for whoever asked for it. Anybody
// else is told there is none.
func (h *exportsHandler) own(c echo.Context) (*repositories.Export, int) {
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
func (h *exportsHandler) Read(c echo.Context) error {
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
func (h *exportsHandler) Delete(c echo.Context) error {
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
