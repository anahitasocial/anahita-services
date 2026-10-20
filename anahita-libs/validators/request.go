package validators

import (
	"anahita-libs/requests"
	"net/http"
	"regexp"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type RequestValidator struct {
	Validator *validator.Validate
}

func NewRequestValidator(v *validator.Validate) *RequestValidator {
	// Register custom validators
	v.RegisterValidation("search_query", validateSearchQuery)
	v.RegisterValidation("name_search_query", validateNameSearchQuery)
	v.RegisterValidation("username", validateUsername)
	v.RegisterValidation("hashtag", validateHashtagAlias)
	v.RegisterValidation("actor_add_permission_access", validateActorAddPermissionAccess)
	v.RegisterValidation("person_add_permission_access", validatePersonAddPermissionAccess)

	return &RequestValidator{
		Validator: v,
	}
}

func (cv *RequestValidator) Validate(i interface{}) error {
	if err := cv.Validator.Struct(i); err != nil {
		// Optionally, you could return the error to give each route more control over the status code
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

// A search: letters, marks and numbers in any language ("Montréal", "Zürich",
// "東京"), spaces, and what people type in a name, a handle or the address of
// a profile on another server ("O'Brien", "@ana@example.org",
// "https://pixelfed.social/dansup"). It refused everything but unaccented
// Latin letters, so a search in French or German, or for an address, was
// answered 400 (the user, 2026-10-19). Nothing here is written into SQL: the
// words are bound parameters, LIKE's wildcards are taken out
// (sanitizeSearchString) and a regular expression made of them is quoted
// (regexp.QuoteMeta).
var searchQuery = regexp.MustCompile(`^[\p{L}\p{M}\p{N} \-_.,+#@"'’:/?&=~!()]+$`)

// A search among names: the same letters, with no address in it.
var nameSearchQuery = regexp.MustCompile(`^[\p{L}\p{M}\p{N} \-_.+"'’]+$`)

// validateSearchQuery is a search of words, names, handles or addresses.
func validateSearchQuery(fl validator.FieldLevel) bool {
	return searchQuery.MatchString(fl.Field().String())
}

// validateNameSearchQuery is a search among names.
func validateNameSearchQuery(fl validator.FieldLevel) bool {
	return nameSearchQuery.MatchString(fl.Field().String())
}

// validateUsername
// field only contain characters that are deemed safe for usernames
func validateUsername(fl validator.FieldLevel) bool {
	str := fl.Field().String()
	match, _ := regexp.MatchString(`^[a-zA-Z0-9\-_]+$`, str)

	return match
}

// validateHashtagAlias
// field only contains utf8 characters, underscores, and emojis
func validateHashtagAlias(fl validator.FieldLevel) bool {
	str := fl.Field().String()
	// Regex pattern to include UTF-8 characters (letters and numbers), underscores, and specified emoji ranges
	match, _ := regexp.MatchString(`^[\p{L}0-9_]+|[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{1F900}-\x{1F9FF}]+$`, str)

	return match
}

// validateActorAddPermissionAccess validates the access value based on the service
func validateActorAddPermissionAccess(fl validator.FieldLevel) bool {
	// Get access value from the field
	access := fl.Field().String()

	// Get the parent struct to read the service name
	addPerm, ok := fl.Parent().Interface().(requests.ActorEditAddPermission)
	if !ok {
		return false
	}

	// Only values permissions.AddAccessAllows enforces. A value that passes
	// here and matches nothing there refuses everybody without saying so.
	switch addPerm.Entity {
	case "comment":
		// Registered or followers, and only on a public or registered
		// group — on a narrower one the group's access decides. See
		// permissions.CommentAccess.
		return access == "registered" || access == "followers"

	case "like":
		return access == "registered" || access == "followers" || access == "admins"

	default:
		// Posts, and who can add followers. A group follows nobody, so
		// leaders and mutuals have nothing to match.
		return access == "followers" || access == "admins"
	}
}

// validatePersonAddPermissionAccess validates the access value based on the service
func validatePersonAddPermissionAccess(fl validator.FieldLevel) bool {
	// Get access value from the field
	access := fl.Field().String()

	// Get the parent struct to read the service name
	addPerm, ok := fl.Parent().Interface().(requests.PersonEditAddPermission)
	if !ok {
		return false
	}

	// Only values permissions.AddAccessAllows enforces. A value that passes
	// here and matches nothing there refuses everybody without saying so.
	switch addPerm.Entity {
	case "comment":
		// Registered or followers, and only on a public or registered
		// profile — on a narrower one the profile's access decides. See
		// permissions.CommentAccess.
		return access == "registered" || access == "followers"

	case "like", "mention":
		// Who may like what is here, and who may mention the person
		// (permissions.CanMention): the same levels.
		return access == "registered" || access == "followers" || access == "leaders" || access == "mutuals" || access == "admins"

	case "tag":
		// Who may tag the person in a picture or a video (roadmap row 103):
		// anybody signed in, each approved first, or nobody. See
		// permissions.TagDecision.
		return access == "registered" || access == "approve" || access == "admins"

	case "follower":
		// Not enforced for people — anybody may follow a person — so the
		// one value is the one the defaults carry.
		return access == "admins"

	default:
		// Posts. admins on a person is the person themselves.
		return access == "followers" || access == "leaders" || access == "mutuals" || access == "admins"
	}
}
