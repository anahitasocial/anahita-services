package services

import (
	"auth/data/entities"
	"strconv"
	"strings"

	libs_services "anahita-libs/data/services"
	libs_responses "anahita-libs/responses"
)

// GetUserInfo returns the OIDC userinfo claims for the authenticated
// viewer. Returns a hybrid payload: the standard OIDC claims (sub, email,
// name, preferred_username) for OIDC-conformant clients, plus the actor
// shape client-side apps expect.
//
// The non-standard half is named like the rest of the platform — `alias`,
// `person_type` — not like the OIDC half. Only a claim an external
// specification actually names keeps protocol vocabulary, and
// preferred_username is the only one here that does. `username` and
// `usertype` were neither: they were leftovers, and because this endpoint
// is the ONLY one whose response the web app does not run through its
// camel-casing interceptor, they were leftovers nothing else in the client
// agreed with.
//
// Field selection is scope-gated per OIDC §5.4:
//   - "openid"  → sub (always)
//   - "profile" → name, preferred_username, and the actor fields
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
		info["alias"] = user.Alias              // custom, and what every other actor calls it

		// Avatar, keyed by size and shaped like every other node
		// response so clients can reuse the same rendering path.
		//
		// The stored value is only an object key; the bucket is private,
		// so the URLs have to be presigned or the browser gets a 403.
		// Skipped entirely when the person has no avatar — otherwise
		// every size would carry an empty, pointlessly signed URL.
		if user.GetAvatarFilename() != "" {
			libs_services.PresignAvatar(s.Srv.FilesGRPC, user)
			info["avatar_urls"] = libs_responses.NewAvatar(user.Avatar)
		}

		// Anahita-specific actor fields. Snake_case to match the rest of
		// the platform's JSON; the web app camel-cases them on the way in,
		// as it does every other response.
		info["id"] = user.ID
		info["type"] = user.Type
		info["enabled"] = user.Enabled

		// The viewer's role. Clients gate their authenticated routes on
		// this — a viewer whose cached profile is missing it reads as an
		// unauthenticated guest and gets bounced to the login screen.
		//
		// `person_type`, matching nodes.person_type and every other actor
		// payload. As `usertype` it did not match, and the web app's
		// permission helpers all read the actor spelling — so every
		// administration control in the client was off for everybody,
		// super administrators included.
		info["person_type"] = user.PersonType
		info["totp_enabled"] = user.TotpEnabled
		info["last_visited_at"] = user.LastVisitedAt

		// What this person has accepted, one entry per document.
		//
		// The web app compares each version against the one in
		// statics/legal/index.js and sends anybody behind to the agreements
		// page. Shaped as an object per document rather than two flat fields,
		// so a third document is a third key and not a naming scheme.
		//
		// The audit trio — ip, user agent, os — is deliberately NOT here. It is
		// evidence for a person reading the row, not input to anything the
		// client decides, and it has no business in a token-bearing response.
		info["tos_agreement"] = agreementClaim(user.TermsOfService)
		info["privacy_agreement"] = agreementClaim(user.PrivacyPolicy)
		// The site's rules, and the version agreed to, which the web app
		// compares with the one GET /site has now (roadmap row 102).
		info["rules_agreement"] = agreementClaim(user.SiteRules)

		// Onboarding. The web app sends a person with an incomplete profile
		// through it after the agreements, unless they have been through it
		// before — completed or skipped.
		//
		// onboarded_at is the timestamp, or null when they have not been. A
		// nil *time.Time boxed in an `any` marshals as null, which is what
		// "not yet" should say.
		//
		// has_bio rather than the bio. The gate needs to know whether the
		// profile is incomplete without a second request, and avatar_urls
		// above already answers half of that by being absent; this is the
		// other half. The text itself is the profile's, and is read from the
		// profile.
		info["onboarded_at"] = user.OnboardedAt
		info["has_bio"] = user.HasBio

		// The language this person usually posts in, as a tag such as "fr".
		// The composer starts on it. Left out when they have not said, and
		// the web app then goes by the browser's language.
		if user.Language != "" {
			info["language"] = user.Language
		}
	}

	if scopeSet["email"] {
		info["email"] = user.Email
	}

	return info
}

// agreementClaim is the part of a consent record the web app needs to decide
// whether to ask again.
func agreementClaim(c entities.Consent) map[string]any {
	return map[string]any{
		"accepted": c.Accepted,
		"version":  c.Version,
	}
}
