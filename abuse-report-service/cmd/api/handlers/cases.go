package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"abusereport/constants"
	"abusereport/data/entities"
	"abusereport/data/repositories"
	"abusereport/requests"
	"abusereport/responses"

	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

type casesHandler struct {
	*Env
}

func (h *casesHandler) mapper(c echo.Context, ids []int64) *responses.Mapper {
	return &responses.Mapper{
		Nodes:     h.readNodes(ids),
		Catalogue: h.Reasons.Catalogue(),
		Language:  language(c),
	}
}

// caseNodeIDs are the nodes a case refers to: what was reported, who answers
// for it, and who decided.
func caseNodeIDs(list ...*entities.Case) []int64 {
	var ids []int64
	for _, item := range list {
		ids = append(ids, item.TargetID, item.TargetOwnerID, item.DecidedBy)
	}
	return ids
}

// Browse lists cases of one status, the most recently reported first.
//
//	GET /abuse-reports/cases?status=open&reason=spam&limit=20&offset=0
//
// `reason` is optional: with it, only cases in which somebody gave that
// reason. A key that is not on the list is not an error. It may be one the
// installation has since removed, and it simply matches what it matches.
func (h *casesHandler) Browse(c echo.Context) error {
	if err := h.Perms.CanBrowseCases(getViewer(c)); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	status := c.QueryParam("status")
	if status == "" {
		status = constants.CaseStatusOpen
	}
	if !slices.Contains(constants.CaseStatuses, status) {
		return c.NoContent(http.StatusBadRequest)
	}

	reasonKey := strings.TrimSpace(c.QueryParam("reason"))
	if len(reasonKey) > 64 {
		return c.NoContent(http.StatusBadRequest)
	}

	limit, _ := strconv.ParseUint(c.QueryParam("limit"), 10, 64)
	offset, _ := strconv.ParseUint(c.QueryParam("start"), 10, 64)
	if limit == 0 {
		limit = constants.DefaultBrowseLimit
	}
	if limit > constants.MaxBrowseLimit {
		limit = constants.MaxBrowseLimit
	}

	list, err := h.Repos.Cases.Browse(status, reasonKey, limit, offset)
	if err != nil {
		log.Println("abuse cases browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	total, err := h.Repos.Cases.Count(status, reasonKey)
	if err != nil {
		log.Println("abuse cases browse: count failed:", err)
	}

	caseIDs := make([]int64, 0, len(list))
	for _, item := range list {
		caseIDs = append(caseIDs, item.ID)
	}
	counts, err := h.Repos.Cases.ReasonCounts(caseIDs)
	if err != nil {
		log.Println("abuse cases browse: reason counts failed:", err)
	}

	mapper := h.mapper(c, caseNodeIDs(list...))
	items := make([]responses.Case, 0, len(list))
	for _, item := range list {
		items = append(items, mapper.Case(item, counts))
	}

	return c.JSON(http.StatusOK, libs_responses.List[responses.Case]{
		Data: items,
		Pagination: libs_responses.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

// Summary is how many cases are waiting, for the number on the
// administration menu.
//
//	GET /abuse-reports/cases/summary
func (h *casesHandler) Summary(c echo.Context) error {
	if err := h.Perms.CanBrowseCases(getViewer(c)); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	open, err := h.Repos.Cases.Count(constants.CaseStatusOpen, "")
	if err != nil {
		log.Println("abuse cases summary:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, responses.CaseSummary{Open: open})
}

func (h *casesHandler) readCase(c echo.Context) (*entities.Case, int) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, http.StatusBadRequest
	}

	item, err := h.Repos.Cases.Read(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, http.StatusNotFound
	}
	if err != nil {
		log.Println("abuse case read:", err)
		return nil, http.StatusInternalServerError
	}

	return item, http.StatusOK
}

// Read is one case: what was reported, every report in it and who made it,
// and the earlier cases the same node has had.
//
//	GET /abuse-reports/cases/:id
//
// It also looks at whether the node still exists. The service is told when
// one is deleted, but by an event, and events can be lost: opening the case
// is the check that cannot be.
func (h *casesHandler) Read(c echo.Context) error {
	if err := h.Perms.CanBrowseCases(getViewer(c)); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	item, status := h.readCase(c)
	if item == nil {
		return c.NoContent(status)
	}

	reports, err := h.Repos.Cases.BrowseReports(item.ID)
	if err != nil {
		log.Println("abuse case read: reports:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	previous, err := h.Repos.Cases.BrowseForTarget(item.TargetID, item.ID)
	if err != nil {
		log.Println("abuse case read: earlier cases:", err)
	}

	ids := caseNodeIDs(append([]*entities.Case{item}, previous...)...)
	for _, report := range reports {
		ids = append(ids, report.ReporterID)
	}
	mapper := h.mapper(c, ids)

	if _, exists := mapper.Nodes[item.TargetID]; !exists && item.TargetGoneAt == nil {
		if _, err := h.Repos.Cases.MarkTargetGone(item.TargetID); err != nil {
			log.Println("abuse case read: could not record that the target is gone:", err)
		} else if refreshed, err := h.Repos.Cases.Read(item.ID); err == nil {
			item = refreshed
		}
	}

	allCases := append([]*entities.Case{item}, previous...)
	caseIDs := make([]int64, 0, len(allCases))
	for _, each := range allCases {
		caseIDs = append(caseIDs, each.ID)
	}
	counts, err := h.Repos.Cases.ReasonCounts(caseIDs)
	if err != nil {
		log.Println("abuse case read: reason counts failed:", err)
	}

	detail := responses.CaseDetail{
		Case:     mapper.Case(item, counts),
		Reports:  make([]responses.Report, 0, len(reports)),
		Previous: make([]responses.Case, 0, len(previous)),
	}
	for _, report := range reports {
		detail.Reports = append(detail.Reports, mapper.Report(report))
	}
	for _, earlier := range previous {
		detail.Previous = append(detail.Previous, mapper.Case(earlier, counts))
	}

	return c.JSON(http.StatusOK, detail)
}

// Resolve closes an open case.
//
//	POST /abuse-reports/cases/:id/resolve   { status, action, note }
//
// It records a decision; it does not carry one out. Deleting a post or
// disabling an account is done by the service that owns it, under that
// service's own rules, before this is called.
//
//	204  closed
//	409  the case was not open: somebody else decided it first
func (h *casesHandler) Resolve(c echo.Context) error {
	viewer := getViewer(c)
	if err := h.Perms.CanDecideCases(viewer); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	req := &requests.CaseResolve{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	req.Note = strings.TrimSpace(req.Note)
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// A dismissed case had nothing done about it, whatever was sent. An
	// actioned one with no action named is recorded as "other".
	switch {
	case req.Status == constants.CaseStatusDismissed:
		req.Action = constants.ActionNone
	case req.Action == "" || req.Action == constants.ActionNone:
		req.Action = constants.ActionOther
	}

	item, status := h.readCase(c)
	if item == nil {
		return c.NoContent(status)
	}

	closed, err := h.Repos.Cases.Resolve(item.ID, req.Status, req.Action, req.Note, viewer.ID)
	if err != nil {
		log.Println("abuse case resolve:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !closed {
		return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "already_decided"})
	}

	log.Printf("abuse case %d: %s (%s) by %d", item.ID, req.Status, req.Action, viewer.ID)
	return c.NoContent(http.StatusNoContent)
}

// Reopen opens a closed case again.
//
//	POST /abuse-reports/cases/:id/reopen
//
//	204  reopened
//	409  it is already open, or the node has been reported again since and
//	     has a newer open case, which is the one to use
func (h *casesHandler) Reopen(c echo.Context) error {
	viewer := getViewer(c)
	if err := h.Perms.CanDecideCases(viewer); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	item, status := h.readCase(c)
	if item == nil {
		return c.NoContent(status)
	}

	reopened, err := h.Repos.Cases.Reopen(item.ID)
	if errors.Is(err, repositories.ErrOpenCaseExists) {
		return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "newer_case_open"})
	}
	if err != nil {
		log.Println("abuse case reopen:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if !reopened {
		return c.JSON(http.StatusConflict, libs_responses.Refusal{Error: "already_open"})
	}

	log.Printf("abuse case %d: reopened by %d", item.ID, viewer.ID)
	return c.NoContent(http.StatusNoContent)
}
