package constants

import "strings"

// Who may reply to a post, as its author sets it.
//
// One of the two words that stand alone, or any of the three groups, joined
// by commas. The groups are measured against the post's author, and somebody
// in any one of them may reply:
//
//	anyone      whoever the profile lets reply. What a post has when its
//	            author has not said.
//	nobody      nobody but the people who look after the thread.
//	followers   people who follow the author.
//	following   people the author follows.
//	mentioned   people the post mentions.
//
// It narrows who may reply and never widens it: the profile's own rule for
// who may comment still has to be met first.
const (
	ReplyAccessAnyone    = "anyone"
	ReplyAccessNobody    = "nobody"
	ReplyAccessFollowers = "followers"
	ReplyAccessFollowing = "following"
	ReplyAccessMentioned = "mentioned"
)

// ReplyAccessGroups are the groups, in the order they are stored and sent.
var ReplyAccessGroups = []string{
	ReplyAccessFollowers,
	ReplyAccessFollowing,
	ReplyAccessMentioned,
}

// NormalizeReplyAccess checks a value and returns it in its stored form: the
// groups without repeats, in the order above.
//
// Empty is returned as empty and valid: nothing was said. "anyone" and
// "nobody" stand alone; either beside a group is refused, because it is not
// clear what was meant.
func NormalizeReplyAccess(raw string) (string, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "", true
	}

	if raw == ReplyAccessAnyone || raw == ReplyAccessNobody {
		return raw, true
	}

	chosen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		known := false
		for _, group := range ReplyAccessGroups {
			if part == group {
				known = true
			}
		}
		if !known {
			return "", false
		}
		chosen[part] = true
	}

	var groups []string
	for _, group := range ReplyAccessGroups {
		if chosen[group] {
			groups = append(groups, group)
		}
	}

	return strings.Join(groups, ","), true
}

// EffectiveReplyAccess is a stored value as it is read: "anyone", "nobody",
// or the groups. open is the post's older switch for replies; a post with it
// off reads as "nobody" whatever else it says, which is every post closed
// before authors could choose.
func EffectiveReplyAccess(stored string, open bool) string {
	if !open || stored == ReplyAccessNobody {
		return ReplyAccessNobody
	}

	if stored == "" {
		return ReplyAccessAnyone
	}

	return stored
}

// ReplyAccessHas reports whether a stored value includes a group.
func ReplyAccessHas(access string, group string) bool {
	for _, part := range strings.Split(access, ",") {
		if part == group {
			return true
		}
	}
	return false
}
