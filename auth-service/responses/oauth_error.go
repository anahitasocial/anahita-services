package responses

// OAuthErrorResponse is the standard OAuth error response body per
// RFC 6749 §5.2.
//
// ErrorCode is a non-standard addition. RFC 6749 collapses every refresh
// failure into invalid_grant, which leaves a client unable to tell "your
// session is over, send the user to log in" from "that raced, read your
// token store again" without string-matching the human-readable
// description. Clients should branch on ErrorCode and treat an unknown or
// absent value as terminal.
type OAuthErrorResponse struct {
	// Why, as RFC 6749 names it: `invalid_client`, `invalid_grant` and so on.
	Error string `json:"error"`
	// The same in words.
	Description string `json:"error_description,omitempty"`
	// For a refused refresh token, which it was: `refresh_expired`, `refresh_not_found` and `refresh_reuse` end the session; `refresh_raced` does not, since another request refreshed first.
	ErrorCode string `json:"error_code,omitempty"`
}

// Machine-readable values for OAuthErrorResponse.ErrorCode.
const (
	// OAuthErrorCodeRefreshExpired means the refresh token passed its
	// 30-day lifetime. Terminal: the user must log in again.
	OAuthErrorCodeRefreshExpired = "refresh_expired"

	// OAuthErrorCodeRefreshNotFound means no such refresh token exists.
	// Terminal.
	OAuthErrorCodeRefreshNotFound = "refresh_not_found"

	// OAuthErrorCodeRefreshReuse means an already-rotated refresh token
	// was presented. Terminal, and the user's other sessions for this
	// client have been revoked.
	OAuthErrorCodeRefreshReuse = "refresh_reuse"

	// OAuthErrorCodeRefreshRaced means a concurrent request rotated the
	// token first. NOT terminal — the caller should re-read its token
	// store, where the winning request's fresh pair is already stored.
	// Clients must not clear credentials on this code.
	OAuthErrorCodeRefreshRaced = "refresh_raced"
)
