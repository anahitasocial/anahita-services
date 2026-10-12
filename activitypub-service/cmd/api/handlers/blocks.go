package handlers

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"activitypub/data/repositories"

	"github.com/labstack/echo/v4"
)

// The servers this installation will have nothing to do with.
//
// A list kept by super administrators, with the other settings that are
// the whole installation's: refusing a server is a choice made for
// everybody on it, not a decision about one report. A server on it is not
// listened to (what it
// sends is refused before its key is even fetched), is sent nothing, and
// nobody on it can be looked up. A name covers every name under it.
//
// Refusing a server does not remove what already came from it: people from
// it who are kept here stay, and so do their posts and the follows between
// them and people here; nothing more arrives and nothing more is sent.
// Removing them is done one by one, as any post or person is removed.

type blockRequest struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

var domainShape = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)

// BrowseBlocks lists the servers that are refused.
func (h *Handlers) BrowseBlocks(c echo.Context) error {
	if !getViewer(c).IsSuperAdmin() {
		return c.NoContent(http.StatusForbidden)
	}
	blocks, err := h.Blocks.Browse()
	if err != nil {
		log.Println("activitypub blocks:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, map[string]any{"data": blocks})
}

// AddBlock refuses a server, or changes why it is refused.
func (h *Handlers) AddBlock(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.IsSuperAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	req := &blockRequest{}
	if err := c.Bind(req); err != nil {
		return refuse(c, http.StatusBadRequest, "domain_required")
	}

	domain := repositories.CleanDomain(req.Domain)
	if len(domain) > 255 || !domainShape.MatchString(domain) {
		return refuse(c, http.StatusBadRequest, "domain_unreadable")
	}
	// Not this installation itself, by either of its names.
	if h.isOurDomain(domain) {
		return refuse(c, http.StatusBadRequest, "domain_is_here")
	}

	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}

	block := &repositories.DomainBlock{Domain: domain, Reason: reason, CreatedBy: viewer.ID}
	if err := h.Blocks.Add(block); err != nil {
		log.Println("activitypub blocks:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusCreated, block)
}

// DeleteBlock stops refusing a server.
func (h *Handlers) DeleteBlock(c echo.Context) error {
	if !getViewer(c).IsSuperAdmin() {
		return c.NoContent(http.StatusForbidden)
	}
	domain := repositories.CleanDomain(c.Param("domain"))
	if !domainShape.MatchString(domain) {
		return c.NoContent(http.StatusNotFound)
	}
	if err := h.Blocks.Delete(domain); err != nil {
		log.Println("activitypub blocks:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.NoContent(http.StatusNoContent)
}
