package handlers

import (
	"net/http"

	"abusereport/responses"

	"github.com/labstack/echo/v4"
)

type reasonsHandler struct {
	*Env
}

// Browse lists the reasons somebody can give for reporting a node of one
// type, in their language.
//
//	GET /abuse-reports/reasons?type=<node type>
//
// With no type, every reason. Signed in only: the list is not secret, but
// there is nothing a signed-out visitor can do with it.
func (h *reasonsHandler) Browse(c echo.Context) error {
	if err := h.Perms.CanReport(getViewer(c)); err != nil {
		return c.NoContent(http.StatusForbidden)
	}

	catalogue := h.Reasons.Catalogue()

	list := catalogue.Reasons
	if nodeType := c.QueryParam("type"); nodeType != "" {
		list = catalogue.For(nodeType)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"data": responses.MapReasons(list, language(c)),
	})
}
