package handlers

import (
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"abusereport/constants"
	"abusereport/responses"

	libs_entities "anahita-libs/data/entities"
	libs_responses "anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// BrowseForProfile is a group's or an event's own reports, for its
// administrators (roadmap row 102, part 6b): the cases with a report against
// its rules, and those reports, with nobody's name. Never a report on the
// group or event itself, nor on what the viewer is answerable for: whoever
// is reported is not told. They act on what was
// reported where it is, by deleting or hiding it; deciding the case stays
// with the site's administrators, whose case it is.
//
//	GET /abuse-reports/profiles/:id/cases?status=open
//
//	200  the cases, newest first
//	403  the viewer administers neither it nor the site
//	404  there is no such group or event
func (h *casesHandler) BrowseForProfile(c echo.Context) error {
	viewer := getViewer(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}

	profile, err := h.Srv.GraphNodeGRPC.Read(id)
	if err != nil || profile == nil || !hasRules(profile) {
		return c.NoContent(http.StatusNotFound)
	}
	if !h.administers(viewer, profile) {
		return c.NoContent(http.StatusForbidden)
	}

	status := c.QueryParam("status")
	if status == "" {
		status = constants.CaseStatusOpen
	}
	if !slices.Contains(constants.CaseStatuses, status) {
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

	list, err := h.Repos.Cases.BrowseForProfile(id, viewer.ID, status, limit, offset)
	if err != nil {
		log.Println("abuse cases of a profile:", err)
		return c.NoContent(http.StatusInternalServerError)
	}
	total, err := h.Repos.Cases.CountForProfile(id, viewer.ID, status)
	if err != nil {
		log.Println("abuse cases of a profile: count failed:", err)
	}

	caseIDs := make([]int64, 0, len(list))
	targets := make([]int64, 0, len(list))
	for _, item := range list {
		caseIDs = append(caseIDs, item.ID)
		targets = append(targets, item.TargetID)
	}
	reports, err := h.Repos.Cases.ProfileReports(id, caseIDs)
	if err != nil {
		log.Println("abuse cases of a profile: reports failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Only what was reported is read: nobody's name goes out from here.
	mapper := &responses.Mapper{Nodes: h.readNodes(targets)}
	items := make([]responses.ProfileCase, 0, len(list))
	for _, item := range list {
		items = append(items, mapper.ProfileCase(item, reports))
	}

	return c.JSON(http.StatusOK, libs_responses.List[responses.ProfileCase]{
		Data: items,
		Pagination: libs_responses.Pagination{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	})
}

// hasRules is whether a node is a group or an event: what has rules.
func hasRules(node *libs_entities.AnahitaNode) bool {
	return strings.Contains(node.Type, ".group.") || strings.Contains(node.Type, ".event.")
}

// administers is whether the viewer administers the profile, or the site.
func (h *casesHandler) administers(viewer *libs_entities.Viewer, profile *libs_entities.AnahitaNode) bool {
	if h.Perms.CanBrowseCases(viewer) == nil {
		return true
	}
	rel, err := h.Srv.SocialgraphGRPC.Read(profile.ID, viewer.ID)
	if err != nil {
		log.Println("abuse cases of a profile: relation:", err)
		return false
	}
	return rel != nil && rel.HasAdmin
}
