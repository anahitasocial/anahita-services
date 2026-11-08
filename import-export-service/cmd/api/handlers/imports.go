package handlers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"anahita-libs/apidoc"
	"anahita-libs/constants"
	libs_responses "anahita-libs/responses"

	"importexport/data/repositories"
	"importexport/importfile"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Imports: a person brings their lists in from a file (anahita-web roadmap
// row 124). The file is read at once and what is in it shown; nothing is
// done until the person starts it, with the kinds they chose; then the
// worker goes through it a line at a time, as the person would by hand.

type ImportsHandler struct {
	*Env
}

func NewImportsHandler(env *Env) *ImportsHandler {
	return &ImportsHandler{env}
}

// KindCount is how one kind of line in an import is getting on.
type KindCount struct {
	// "follow", "block", "mute" or "save".
	Kind string `json:"kind"`
	// How many lines of the kind the file had.
	Total int `json:"total"`
	// Done, with nothing to do (followed or blocked already), not done and
	// listed with why, and not chosen.
	Done    int `json:"done"`
	Already int `json:"already"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	// Waiting for the worker.
	Left int `json:"left"`
}

// ImportItem is one import, as its owner sees it.
type ImportItem struct {
	ID string `json:"id"`
	// "ready" (read, waiting to be started), "running", "done" or
	// "cancelled".
	Status string `json:"status"`
	// "anahita" for another Anahita's download, "mastodon" for a CSV.
	Source     string      `json:"source"`
	FileName   string      `json:"file_name"`
	CreatedAt  time.Time   `json:"created_at"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Kinds      []KindCount `json:"kinds"`
}

// ImportList is the viewer's latest imports, newest first.
type ImportList struct {
	Data []*ImportItem `json:"data"`
}

type ImportRequest struct {
	// The import's id.
	ID string `param:"id"`
}

// ImportUpload describes the form an import is sent in.
type ImportUpload struct {
	// The file: another Anahita's download (zip), or a CSV.
	File apidoc.File `form:"file" validate:"required"`
	// What a CSV with no header is: "follow", "block", "mute" or "save".
	// Asked of the person when the file says nothing (`kind_unknown`).
	Kind string `form:"kind"`
}

// ImportStart is what the viewer chose to bring in.
type ImportStart struct {
	ID string `param:"id"`
	// The kinds to bring in, of those the file has: "follow", "block",
	// "mute", "save".
	Kinds []string `json:"kinds"`
}

func (h *ImportsHandler) respond(imp *repositories.Import) (*ImportItem, error) {
	counts, err := h.Imports.Counts(imp.ID)
	if err != nil {
		return nil, err
	}
	byKind := map[string]*KindCount{}
	for _, kind := range repositories.Kinds {
		byKind[kind] = &KindCount{Kind: kind}
	}
	for _, c := range counts {
		k, ok := byKind[c.Kind]
		if !ok {
			continue
		}
		k.Total += c.N
		switch c.Status {
		case repositories.LineDone:
			k.Done += c.N
		case repositories.LineAlready:
			k.Already += c.N
		case repositories.LineFailed:
			k.Failed += c.N
		case repositories.LineSkipped:
			k.Skipped += c.N
		default:
			k.Left += c.N
		}
	}
	item := &ImportItem{
		ID:         imp.ID,
		Status:     imp.Status,
		Source:     imp.Source,
		FileName:   imp.FileName,
		CreatedAt:  imp.CreatedAt,
		StartedAt:  imp.StartedAt,
		FinishedAt: imp.FinishedAt,
		Kinds:      []KindCount{},
	}
	for _, kind := range repositories.Kinds {
		if byKind[kind].Total > 0 {
			item.Kinds = append(item.Kinds, *byKind[kind])
		}
	}
	return item, nil
}

// own is the viewer's import by its id; nil when it is not theirs.
func (h *ImportsHandler) own(c echo.Context, id string) (*repositories.Import, error) {
	imp, err := h.Imports.Read(id)
	if err != nil || imp == nil || imp.PersonID != getViewer(c).ID {
		return nil, err
	}
	return imp, nil
}

// Browse is the viewer's latest imports.
func (h *ImportsHandler) Browse(c echo.Context) error {
	imports, err := h.Imports.OfPerson(getViewer(c).ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	list := &ImportList{Data: []*ImportItem{}}
	for _, imp := range imports {
		item, err := h.respond(imp)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		list.Data = append(list.Data, item)
	}
	return c.JSON(http.StatusOK, list)
}

// Add reads an uploaded file, the form's `file`, and keeps what is in it,
// waiting to be started. `kind` says what a CSV with no header is.
func (h *ImportsHandler) Add(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.HasScope(constants.OAuthScopeSocialgraphWrite) {
		return c.JSON(http.StatusForbidden, libs_responses.Refusal{Error: "insufficient_scope"})
	}

	header, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: importfile.ErrEmpty.Error()})
	}
	if header.Size > importfile.MaxBytes {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: importfile.ErrTooLarge.Error()})
	}
	file, err := header.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: importfile.ErrUnreadable.Error()})
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, importfile.MaxBytes+1))
	if err != nil {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: importfile.ErrUnreadable.Error()})
	}

	source, lines, err := importfile.Read(header.Filename, data, c.FormValue("kind"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, libs_responses.Refusal{Error: err.Error()})
	}

	imp := &repositories.Import{
		ID:       uuid.NewString(),
		PersonID: viewer.ID,
		Source:   source,
		FileName: header.Filename,
	}
	if err := h.Imports.Add(imp, lines); err != nil {
		if errors.Is(err, repositories.ErrImportActive) {
			return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "active"})
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	item, err := h.respond(imp)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusCreated, item)
}

// Read is how an import is getting on.
func (h *ImportsHandler) Read(c echo.Context) error {
	imp, err := h.own(c, c.Param("id"))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if imp == nil {
		return c.NoContent(http.StatusNotFound)
	}
	item, err := h.respond(imp)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, item)
}

// Start sets an import going, with the kinds the viewer chose.
func (h *ImportsHandler) Start(c echo.Context) error {
	req := &ImportStart{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	known := map[string]bool{}
	for _, kind := range repositories.Kinds {
		known[kind] = true
	}
	for _, kind := range req.Kinds {
		if !known[kind] {
			return c.NoContent(http.StatusBadRequest)
		}
	}

	imp, err := h.own(c, req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if imp == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if err := h.Imports.Start(imp.ID, req.Kinds); err != nil {
		if errors.Is(err, repositories.ErrImportNotInState) {
			return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "not_ready"})
		}
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	imp, err = h.Imports.Read(imp.ID)
	if err != nil || imp == nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	item, err := h.respond(imp)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, item)
}

// Delete gives an import up: what it has not done is not done. What it did
// stays done.
func (h *ImportsHandler) Delete(c echo.Context) error {
	imp, err := h.own(c, c.Param("id"))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if imp == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if err := h.Imports.Cancel(imp.ID); err != nil && !errors.Is(err, repositories.ErrImportNotInState) {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusNoContent)
}

// Failed is what could not be done, as a CSV to keep: the kind, who or
// what, and why in a word.
func (h *ImportsHandler) Failed(c echo.Context) error {
	imp, err := h.own(c, c.Param("id"))
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if imp == nil {
		return c.NoContent(http.StatusNotFound)
	}
	lines, err := h.Imports.Failed(imp.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	var out bytes.Buffer
	w := csv.NewWriter(&out)
	_ = w.Write([]string{"Kind", "Address", "Name", "Reason"})
	for _, line := range lines {
		who := line.Address
		if who == "" {
			who = line.URI
		}
		if who == "" {
			who = line.URL
		}
		_ = w.Write([]string{line.Kind, who, line.Name, line.Reason})
	}
	w.Flush()

	c.Response().Header().Set("Content-Disposition", `attachment; filename="not-imported.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", out.Bytes())
}
