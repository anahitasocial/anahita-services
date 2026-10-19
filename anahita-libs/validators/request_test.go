package validators

import "testing"

// What a search may hold (the user, 2026-10-19): words in any language,
// names with apostrophes, handles and the addresses of profiles elsewhere;
// and not what has no place in one.
func TestSearchQuery(t *testing.T) {
	for _, each := range []struct {
		query string
		may   bool
	}{
		{"garden", true},
		{"Montréal", true},
		{"café au lait", true},
		{"Zürich", true},
		{"東京", true},
		{"O'Brien", true},
		{"O’Brien", true},
		{"#pixelfed", true},
		{"@dansup@pixelfed.social", true},
		{"https://pixelfed.social/dansup", true},
		{"https://mastodon.social/@Gargron", true},
		{"https://example.org/users/ana?x=1&y=2", true},
		{`"exact words"`, true},
		{"a;DROP", false},
		{"<script>", false},
		{"back\\slash", false},
		{"tab\there", false},
	} {
		if got := searchQuery.MatchString(each.query); got != each.may {
			t.Errorf("search %q: %v; want %v", each.query, got, each.may)
		}
	}
}

func TestNameSearchQuery(t *testing.T) {
	for _, each := range []struct {
		query string
		may   bool
	}{
		{"Dax", true},
		{"Zoë", true},
		{"O'Brien", true},
		{"Jean-Luc Picard", true},
		{"https://example.org", false},
		{"@ana", false},
	} {
		if got := nameSearchQuery.MatchString(each.query); got != each.may {
			t.Errorf("name search %q: %v; want %v", each.query, got, each.may)
		}
	}
}
