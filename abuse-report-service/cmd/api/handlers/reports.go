package handlers

import (
	"anahita-libs/responses"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"abusereport/constants"
	"abusereport/data/entities"
	"abusereport/requests"
	libs_entities "anahita-libs/data/entities"

	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

type reportsHandler struct {
	*Env
}

var reportLimit = ratelimit.Limit{Max: constants.ReportsPerHour, Window: time.Hour}

// Add files the viewer's report against a node.
//
//	POST /abuse-reports   { target_id, reason_key, details, rule, rule_profile_id }
//
//	201  reported
//	200  the viewer had already reported it; their report was updated
//	400  the request is malformed, the reason does not apply to this kind
//	     of node, the reason needs an explanation, the rule is not one of
//	     the rules it is under, or it is the viewer's own
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
		return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "own_content"})
	}

	report := entities.Report{
		ReporterID: viewer.ID,
		Details:    req.Details,
		CreatedIP:  c.RealIP(),
	}
	if req.ReasonKey == constants.ReasonRule {
		// Breaking one of the rules everybody ticked: the site's, or those
		// of the group or event it was posted in (roadmap row 102, part 6).
		if refusal := h.ruleBroken(req, node); refusal != "" {
			return c.JSON(http.StatusBadRequest, responses.Refusal{Error: refusal})
		}
		report.ReasonKey = constants.ReasonRule
		report.RuleTitle = strings.TrimSpace(req.RuleTitle)
		report.RuleProfileID = req.RuleProfileID
	} else {
		reason, ok := h.Reasons.Catalogue().Find(req.ReasonKey)
		if !ok || !reason.Applies(node.Type) {
			return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "reason_not_applicable"})
		}
		if reason.RequiresDetails && req.Details == "" {
			return c.JSON(http.StatusBadRequest, responses.Refusal{Error: "details_required"})
		}
		report.ReasonKey = reason.Key
	}

	_, created, err := h.Repos.Cases.AddReport(
		entities.Case{TargetID: node.ID, TargetType: node.Type, TargetOwnerID: owner},
		report,
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

// ruleBroken checks a report that says a rule was broken, and answers why
// it is refused, or "" when it is not. The site's rules (rule_profile_id 0)
// are taken as the reporter read them, from GET /site: this service does
// not read site.json, and a title that is not one of them only misleads the
// administrator reading it, who has the rules in front of them. A group's
// or an event's are checked: the rule must be one of theirs now, and the
// reported thing must be in it, or be it.
func (h *reportsHandler) ruleBroken(req *requests.ReportAdd, node *libs_entities.AnahitaNode) string {
	title := strings.TrimSpace(req.RuleTitle)
	if title == "" {
		return "rule_required"
	}
	if req.RuleProfileID == 0 {
		return ""
	}
	if !h.underRulesOf(node, req.RuleProfileID) {
		return "rule_not_applicable"
	}

	read, err := h.Srv.ActorsGRPC.ReadRules(req.RuleProfileID, 0)
	if err != nil {
		log.Println("abuse report: could not read the rules of", req.RuleProfileID, err)
		return "rule_not_applicable"
	}
	for _, item := range read.GetRules().GetItems() {
		if item.GetTitle() == title {
			return ""
		}
	}
	return "rule_not_applicable"
}

// underRulesOf is whether a node is under a profile's rules: the profile
// itself, something posted in it, or a reply in a thread posted in it.
func (h *reportsHandler) underRulesOf(node *libs_entities.AnahitaNode, profileID int64) bool {
	if node.ID == profileID || node.OwnerID == profileID {
		return true
	}
	if node.RootID == 0 || node.RootID == node.ID {
		return false
	}
	root, err := h.Srv.GraphNodeGRPC.Read(node.RootID)
	return err == nil && root != nil && root.OwnerID == profileID
}
