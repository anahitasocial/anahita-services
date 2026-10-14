package federation

import "testing"

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
