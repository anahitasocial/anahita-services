package services

import (
	"auth/data/entities"
	"strconv"
	"strings"

	libs_services "anahita-libs/data/services"
	libs_responses "anahita-libs/responses"
)

// GetUserInfo returns the OIDC userinfo claims for the authenticated
// viewer. Returns a hybrid payload: standard OIDC claims (sub, email,
// name, given_name, family_name) for OIDC-conformant clients, plus
// the full user shape that client side apps expect.
//
// Field selection is scope-gated per OIDC §5.4:
//   - "openid"  → sub (always)
//   - "profile" → name, given_name, family_name, person fields
//   - "email"   → email
//
// Scopes the client wasn't granted are silently omitted; this is the
// expected behavior, not an error.
func (s *OAuthService) GetUserInfo(user *entities.User, scopes string) map[string]any {
	info := map[string]any{
		"sub": strconv.FormatInt(user.ID, 10), // OIDC standard claim
	}

	scopeSet := make(map[string]bool, 4)
	for _, scope := range strings.Fields(scopes) {
		scopeSet[scope] = true
	}

	if scopeSet["profile"] {
		// `name` only. given_name and family_name are OPTIONAL in OIDC and
		// there is nothing to fill them from: a profile carries one display
		// name, not a Western first/last pair. The first_name and last_name
		// customs go with them.
		info["name"] = strings.TrimSpace(user.Name)

		// The person's alias. Clients key their /people/<alias> URLs on
		// this and the numeric id does not resolve those routes, so a
		// viewer without it cannot look itself up.
		info["preferred_username"] = user.Alias // OIDC standard claim
		info["username"] = user.Alias           // custom

		// Avatar, keyed by size and shaped like every other node
		// response so clients can reuse the same rendering path.
		//
		// The stored value is only an object key; the bucket is private,
		// so the URLs have to be presigned or the browser gets a 403.
		// Skipped entirely when the person has no avatar — otherwise
		// every size would carry an empty, pointlessly signed URL.
		if user.GetFilename() != "" {
			libs_services.PresignAvatar(s.Srv.FilesGRPC, user)
			info["avatar_urls"] = libs_responses.NewAvatar(user.Avatar)
		}

		// Anahita–specific person fields. Snake_case to match
		// the rest of the auth-service JSON conventions; axios
		// interceptors on the frontend convert to camelCase.
		info["id"] = user.ID
		info["type"] = user.Type
		info["enabled"] = user.Enabled

		// The viewer's role. Clients gate their authenticated routes on
		// this — a viewer whose cached profile is missing it reads as an
		// unauthenticated guest and gets bounced to the login screen.
		info["usertype"] = user.PersonType
		info["totp_enabled"] = user.TotpEnabled
		info["last_visit_date"] = user.LastVisitDate
	}

	if scopeSet["email"] {
		info["email"] = user.Email
	}

	return info
}
