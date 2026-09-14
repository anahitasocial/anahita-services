package handlers

import (
	"errors"
	"regexp"

	"auth/data/entities"

	"github.com/labstack/echo/v4"
	"github.com/mileusna/useragent"
)

// Recording what somebody agreed to.
//
// WHO DECIDES THE VERSION. The web app does: statics/legal/index.js carries the
// version of each document, and whoever changes the text raises it. A viewer
// whose accepted version is behind is sent to the agreements page on their next
// request, whether they joined years ago or a minute ago.
//
// WHAT THE SERVER STILL DECIDES is everything that makes the record evidence
// rather than a claim: the timestamp, the IP address, the user agent and the
// operating system. Those are read from the request here, never taken from a
// form. A client can say which version it is accepting; it cannot say when, or
// from where.
//
// The version the client sends is checked for SHAPE only. It is not compared
// against anything, because there is nothing on this side to compare it
// against — and a person who sends a nonsense version for themselves has only
// misrecorded their own acceptance.

// ErrAgreementVersionMalformed is returned for a version that is not a dotted
// number.
var ErrAgreementVersionMalformed = errors.New(
	"agreement version must be a dotted version number, e.g. 1.0 or 2.1.3")

// agreementVersionShape is a dotted numeric version: 1, 1.0, 2.1.3.
//
// Anchored, bounded to three parts, and digits only. The version is compared
// numerically in the web app, part by part, so "v2", "1.0-beta" or "current"
// would compare as something nobody intended.
var agreementVersionShape = regexp.MustCompile(`^[0-9]{1,6}(\.[0-9]{1,6}){0,2}$`)

// consentSource is what the request contributes: not the checkbox, but the
// circumstances.
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
	// precedence auth_log.go uses, so a person's consent row and their sign-in
	// history describe the same device the same way.
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
// an unusual browser into "could not record your agreement".
func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}

	return value[:max]
}

// acceptInto records one acceptance onto a consent, with the version the
// client names and the circumstances the request supplies.
func acceptInto(c echo.Context, consent *entities.Consent, version string) error {
	if !agreementVersionShape.MatchString(version) {
		return ErrAgreementVersionMalformed
	}

	source := newConsentSource(c)
	consent.Accept(version, source.ip, source.userAgent, source.os)

	return nil
}
