package responses

// OAuthErrorResponse is the standard OAuth error response body per
// RFC 6749 §5.2.
type OAuthErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}
