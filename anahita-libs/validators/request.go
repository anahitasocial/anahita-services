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

// validateSearchQuery
// field only contain characters that are deemed safe for search queries
func validateSearchQuery(fl validator.FieldLevel) bool {
	str := fl.Field().String()
	match, _ := regexp.MatchString(`^[a-zA-Z0-9 \-_\.,\+\#\@"]+$`, str)

	return match
}

// validateNameSearchQuery
// field only contain characters that are deemed safe for name search queries
func validateNameSearchQuery(fl validator.FieldLevel) bool {
	str := fl.Field().String()
	match, _ := regexp.MatchString(`^[a-zA-Z0-9 \-_\.\+"]+$`, str)

	return match
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

	switch addPerm.Entity {
	case "like", "comment":
		// These support "registered", "followers", "admins"
		return access == "registered" || access == "followers" || access == "admins"

	default:
		// For person actors, allow additional values
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

	switch addPerm.Entity {
	case "like", "comment":
		// These support "registered", "followers", "admins"
		return access == "registered" || access == "followers" || access == "admins"

	case "follower":
		// Follower permission only supports followers and admins
		return access == "admins"

	default:
		// For person actors, allow additional values
		return access == "followers" || access == "leaders" || access == "mutuals" || access == "admins"
	}
}
