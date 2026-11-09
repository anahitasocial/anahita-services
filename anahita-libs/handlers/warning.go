package handlers

import (
	"net/http"

	"anahita-libs/data/entities"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// A post's content warning (anahita-services row 122,
// docs/plans/content-warnings.md), as a request gives it and as it is kept.

// ErrWarningInvalid is the error a request gets when its warning is not one:
// a kind that is not one of the five, or an Other with no words or too many.
const ErrWarningInvalid = "warning_invalid"

// ErrWarningNotAllowed is the error an author gets when they would change or
// take off a warning a moderator gave their post.
const ErrWarningNotAllowed = "warning_not_allowed"

// RefuseWarning answers a request whose warning is not one: 400.
func RefuseWarning(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, responses.Refusal{
		Error:       ErrWarningInvalid,
		Description: "warning must be \"adult\", \"suggestive\", \"graphic\", \"nudity\" or \"other\", and an \"other\" warning needs warning_text of up to 100 characters",
	})
}

// RefuseWarningNotAllowed answers an author who would change a moderator's
// warning: 403.
func RefuseWarningNotAllowed(c echo.Context) error {
	return c.JSON(http.StatusForbidden, responses.Refusal{
		Error:       ErrWarningNotAllowed,
		Description: "a moderator gave this post its warning, which its author may not change or take off",
	})
}

// SetWarning gives a post the warning a request asks for, and says whether
// it was one (valid) and whether its author may change the one it has
// (allowed): a moderator's warning stays as it is, unless the same is asked
// for again. Nothing is changed unless both are true.
func SetWarning(post *entities.AnahitaNode, kind, text string) (valid, allowed bool) {
	kind, text, valid = entities.NormalizeWarning(kind, text)
	if !valid {
		return false, true
	}
	if post.WarningByModerator && (post.Warning != kind || post.WarningText != text) {
		return true, false
	}
	if !post.WarningByModerator {
		post.Warning = kind
		post.WarningText = text
	}
	return true, true
}
