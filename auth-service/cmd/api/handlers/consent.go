package handlers

import (
	"errors"

	"auth/data/entities"

	"github.com/labstack/echo/v4"
	"github.com/mileusna/useragent"
)

// Capturing what somebody agreed to, shared by every path that creates or
// claims an account: signup, invite-accept, and the set-password page the
// seeded super administrator lands on.
//
// One implementation because the record is only useful if it is the same
// record everywhere. Three handlers each assembling their own would be three
// chances for one of them to record a version it made up, or to forget the
// audit trio, and the difference would not be visible until somebody asked the
// table a question it could no longer answer.

// ErrAgreementVersionStale is returned when the submitted version is not the
// one in force.
var ErrAgreementVersionStale = errors.New("the agreement version submitted is not the current one")

// consentSource is what the request itself contributes: not the checkbox, but
// the circumstances. Taken from the request rather than the form, because a
// field the client fills in is a claim and these are meant to be a record.
type consentSource struct {
	ip        string
	userAgent string
	os        string
}

func newConsentSource(c echo.Context) consentSource {
	raw := c.Request().Header.Get("User-Agent")
	ua := useragent.Parse(raw)

	// X-Client-OS first: the native apps send it explicitly because their
	// User-Agent strings are not browser-shaped and parse to nothing. Same
	// precedence auth_log.go uses, so a person's consent row and their
	// sign-in history describe the same device the same way.
	os := c.Request().Header.Get("X-Client-OS")
	if os == "" {
		os = ua.OS
	}

	return consentSource{
		ip:        getClientIP(c),
		userAgent: truncate(raw, 255),
		os:        truncate(os, 100),
	}
}

// truncate caps a value at the width of its column.
//
// A User-Agent longer than 255 bytes is not unusual, and MySQL under a strict
// sql_mode rejects the row outright rather than trimming it — which would turn
// an unusual browser into "could not create your account".
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}

	return value[:max]
}

// recordAgreements writes both acceptances onto the user, after checking that
// the versions submitted are the ones in force.
//
// The version check is the point of carrying a version at all. Without it,
// somebody who left the signup page open across a revision is recorded as
// having accepted text they were never shown — and the record would say
// otherwise, which is worse than having no record. They are asked again
// instead.
//
// The booleans are not re-checked here: `eq=true` on the request already
// refused an unticked box, and a second check in a different place is a second
// place for the two to disagree.
func (h *Env) recordAgreements(
	c echo.Context,
	user *entities.User,
	tosVersion, ppVersion string,
) error {
	if tosVersion != h.Config.GetTosVersion() || ppVersion != h.Config.GetPrivacyVersion() {
		return ErrAgreementVersionStale
	}

	source := newConsentSource(c)

	user.TermsOfService.Accept(
		h.Config.GetTosVersion(), source.ip, source.userAgent, source.os)
	user.PrivacyPolicy.Accept(
		h.Config.GetPrivacyVersion(), source.ip, source.userAgent, source.os)

	return nil
}
