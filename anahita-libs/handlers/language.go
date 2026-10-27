package handlers

import (
	"anahita-libs/responses"
	"net/http"

	"anahita-libs/language"

	"github.com/labstack/echo/v4"
)

// ErrLanguageInvalid is the error code sent with a 400 when a request names a
// language that is not a tag this installation accepts.
const ErrLanguageInvalid = "language_invalid"

// RequestLanguage reads the language a request gives a post or a person.
//
// Not given is fine, and comes back as "" and true: nothing guesses a
// language for a post whose author did not say. A tag that is given comes
// back in its usual spelling ("FR-ca" as "fr-CA"). One that is not a language
// tag comes back false, for the handler to refuse with RefuseLanguage.
func RequestLanguage(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}

	return language.Normalize(raw)
}

// RefuseLanguage answers a request whose language is not a tag: 400, saying
// which field.
func RefuseLanguage(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, responses.Refusal{
		Error:       ErrLanguageInvalid,
		Description: "language must be a tag such as \"en\" or \"fr-CA\"",
	})
}
