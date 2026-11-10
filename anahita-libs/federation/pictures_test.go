package federation

import (
	"testing"

	"anahita-libs/data/entities"
)

func TestAMastodonPlaceholderHeaderIsNoCover(t *testing.T) {
	for address, want := range map[string]bool{
		"https://mastodon.social/headers/original/missing.png":          true,
		"https://files.example/accounts/headers/000/1/original/abc.jpg": false,
		"https://cosocial.ca/headers/original/MISSING.png":              true,
		"https://activitypub.space/assets/images/cover-default.png":     true,
		"": false,
	} {
		if got := isPlaceholderCover(address); got != want {
			t.Errorf("%q: %v", address, got)
		}
	}
}

// A profile saved again to name its picture keeps what it had, its lock
// among it: a profile elsewhere that approves its followers is still asked,
// not followed (roadmap row 130).
func TestAPictureNamedKeepsTheProfileLocked(t *testing.T) {
	kept := entities.NewAnahitaNode()
	kept.Type = "node.actor.activitypub-service.person.v1"
	kept.URI, kept.OriginDomain = "https://beige.party/users/medea", "beige.party"
	kept.Name, kept.Alias, kept.Body, kept.WebsiteURL = "Medea", "medea@beige.party", "Words", "https://beige.party/@medea"
	kept.AllowFollowRequest = true

	update := profileAsKept(kept)
	if !update.AllowFollowRequest {
		t.Error("the lock was dropped")
	}
	if update.URI != kept.URI || update.Alias != kept.Alias || update.Name != kept.Name || update.Body != kept.Body {
		t.Errorf("not as kept: %+v", update)
	}
}
