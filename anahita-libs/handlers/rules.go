package handlers

import (
	"net/http"

	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RulesNotAgreed is a group or an event with rules the viewer has not
// agreed to at the version they are at now (roadmap row 102, part 5b): what
// a follow, a request to follow, an event's answer or an invitation's
// acceptance is refused with, until they tick them.
const RulesNotAgreed = "rules_not_agreed"

// IsRulesNotAgreed says whether a graph call was refused for that reason.
func IsRulesNotAgreed(err error) bool {
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.FailedPrecondition && s.Message() == RulesNotAgreed
}

// RefuseRulesNotAgreed answers 409 `rules_not_agreed`: read the rules, tick
// each, agree, and ask again.
func RefuseRulesNotAgreed(c echo.Context) error {
	return c.JSON(http.StatusConflict, responses.Refusal{Error: RulesNotAgreed})
}
