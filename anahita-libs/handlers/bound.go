package handlers

import (
	"log"

	"github.com/labstack/echo/v4"
)

// Bound reads a request into req and checks it by its validate tags,
// logging why when either fails (roadmap row 170: written out by hand in
// every handler, ten lines each time). A false answer is the caller's to
// turn into 400:
//
//	if !Bound(c, req) {
//		return c.NoContent(http.StatusBadRequest)
//	}
func Bound(c echo.Context, req any) bool {
	if err := c.Bind(req); err != nil {
		log.Println(err)
		return false
	}
	if err := c.Validate(req); err != nil {
		log.Println(err)
		return false
	}
	return true
}
