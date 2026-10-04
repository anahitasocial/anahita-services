package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"abusereport/constants"
	"abusereport/data/entities"
	"abusereport/requests"

	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

type reportsHandler struct {
	*Env
}

var reportLimit = ratelimit.Limit{Max: constants.ReportsPerHour, Window: time.Hour}

// Add files the viewer's report against a node.
//
//	POST /abuse-reports   { target_id, reason_key, details }
//
//	201  reported
//	200  the viewer had already reported it; their report was updated
//	400  the request is malformed, the reason does not apply to this kind
//	     of node, the reason needs an explanation, or it is the viewer's own
//	404  there is no such node that the viewer can see
//	429  too many reports in the last hour
//
// A node the viewer cannot see answers exactly as one that does not exist.
// Otherwise reporting would be a way to find out what is hidden.
//
// Nobody is told. The person reported never learns of the report or who made
// it, and administrators find it in their queue.
func (h *reportsHandler) Add(c echo.Context) error {
	viewer := getViewer(c)
	if err := h.Perms.CanReport(viewer); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	// Counted before anything is read, so every attempt costs one, including
	// the ones that are refused below.
	if ratelimit.Exceeded(h.Limiter, constants.RateLimitScopeReport,
		strconv.FormatInt(viewer.ID, 10), reportLimit) {
		return c.NoContent(http.StatusTooManyRequests)
	}

	req := &requests.ReportAdd{}
	if err := c.Bind(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	req.Details = strings.TrimSpace(req.Details)
	if err := c.Validate(req); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	// May the viewer see it? One question, for every kind of node.
	visible, err := h.Srv.GraphNodeGRPC.FilterVisible([]int64{req.TargetID}, viewer)
	if err != nil {
		log.Println("abuse report: visibility check failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	if len(visible) == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	node, err := h.Srv.GraphNodeGRPC.Read(req.TargetID)
	if err != nil || node == nil {
		return c.NoContent(http.StatusNotFound)
	}

	// Reporting yourself, or something you wrote, is not a report.
	owner := responsibleFor(node)
	if owner == viewer.ID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "own_content"})
	}

	reason, ok := h.Reasons.Catalogue().Find(req.ReasonKey)
	if !ok || !reason.Applies(node.Type) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "reason_not_applicable"})
	}
	if reason.RequiresDetails && req.Details == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "details_required"})
	}

	_, created, err := h.Repos.Cases.AddReport(
		entities.Case{TargetID: node.ID, TargetType: node.Type, TargetOwnerID: owner},
		entities.Report{
			ReporterID: viewer.ID,
			ReasonKey:  reason.Key,
			Details:    req.Details,
			CreatedIP:  c.RealIP(),
		},
	)
	if err != nil {
		log.Println("abuse report: could not store the report:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if created {
		return c.NoContent(http.StatusCreated)
	}
	return c.NoContent(http.StatusOK)
}
