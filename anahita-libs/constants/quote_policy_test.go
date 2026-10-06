package constants

import "testing"

func TestNormalizeQuotePolicy(t *testing.T) {
	for raw, want := range map[string]string{"": "", "anyone": "anyone", " Followers ": "followers", "nobody": "nobody"} {
		if got, ok := NormalizeQuotePolicy(raw); !ok || got != want {
			t.Errorf("NormalizeQuotePolicy(%q) = %q, %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"public", "following", "anyone,followers"} {
		if _, ok := NormalizeQuotePolicy(raw); ok {
			t.Errorf("NormalizeQuotePolicy(%q) was accepted", raw)
		}
	}
}

func TestAPostsOwnPolicyComesBeforeItsAuthors(t *testing.T) {
	cases := []struct{ post, author, want string }{
		{"", "", "anyone"},
		{"", "nobody", "nobody"},
		{"anyone", "nobody", "anyone"},
		{"followers", "", "followers"},
	}
	for _, c := range cases {
		if got := EffectiveQuotePolicy(c.post, c.author); got != c.want {
			t.Errorf("EffectiveQuotePolicy(%q, %q) = %q, want %q", c.post, c.author, got, c.want)
		}
	}
}
