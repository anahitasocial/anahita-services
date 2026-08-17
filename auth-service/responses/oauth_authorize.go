package responses

import "net/url"

// OAuthAuthorizeResponse builds OAuth authorization-flow redirect URLs.
// The methods URL-escape parameters and append them properly so query
// strings like "?error=access_denied&error_description=user%20denied"
// are well-formed.

type OAuthAuthorizeResponse struct{}

// BuildRedirect constructs the success redirect to the client's
// registered redirect_uri with the authorization code (and optional
// state) appended as query parameters.
func (r *OAuthAuthorizeResponse) BuildRedirect(redirectURI string, code string, state string) string {
	q := url.Values{}
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	return appendQuery(redirectURI, q)
}

// BuildErrorRedirect constructs the error redirect per RFC 6749 §4.1.2.1
// — the error code and description (and state) are appended as query
// parameters to the client's redirect_uri.
func (r *OAuthAuthorizeResponse) BuildErrorRedirect(redirectURI string, state string, errCode string, errDesc string) string {
	q := url.Values{}
	q.Set("error", errCode)
	if errDesc != "" {
		q.Set("error_description", errDesc)
	}
	if state != "" {
		q.Set("state", state)
	}
	return appendQuery(redirectURI, q)
}

// appendQuery appends url.Values to a URL, handling URLs that already
// have a query string vs. those that don't.
func appendQuery(base string, q url.Values) string {
	if len(q) == 0 {
		return base
	}
	sep := "?"
	for _, c := range base {
		if c == '?' {
			sep = "&"
			break
		}
	}
	return base + sep + q.Encode()
}
