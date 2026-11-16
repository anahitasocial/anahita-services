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

// IsWarningModerator says whether the viewer gives a post its warning as a
// moderator (row 122, phase 4): a site administrator who did not write it.
// A moderator may warn any post, a reply or one from another server too,
// and its author may not change what they gave.
func IsWarningModerator(post *entities.AnahitaNode, viewer *entities.Viewer) bool {
	if viewer == nil || !viewer.IsAdminOrSuperAdmin() {
		return false
	}
	return post.CreatedBy != viewer.ID
}

// SetWarningAsModerator gives a post the warning a moderator asks for, or
// takes it off, whoever gave the one it has. valid is false when the
// warning is not one; nothing is changed then.
func SetWarningAsModerator(post *entities.AnahitaNode, kind, text string) (valid bool) {
	kind, text, valid = entities.NormalizeWarning(kind, text)
	if !valid {
		return false
	}
	post.Warning = kind
	post.WarningText = text
	post.WarningByModerator = kind != ""
	return true
}

// SetWarningBy gives a post the warning a request asks for, as its author
// or, for a moderator (IsWarningModerator), as the site's: the same answers
// as SetWarning. A moderator is always allowed.
func SetWarningBy(post *entities.AnahitaNode, moderator bool, kind, text string) (valid, allowed bool) {
	if moderator {
		return SetWarningAsModerator(post, kind, text), true
	}
	return SetWarning(post, kind, text)
}
