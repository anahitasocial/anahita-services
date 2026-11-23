package feeds

import "testing"

func TestSlugAndPage(t *testing.T) {
	for name, want := range map[string]string{
		"Arts & Design":     "arts-design",
		"  Café au lait!  ": "cafe-au-lait",
		"Ceres Station 2":   "ceres-station-2",
		"東京":                "",
		"Düsseldorf – Köln": "dusseldorf-koln",
		"a very long name indeed that goes on and on and on past what a link should hold": "a-very-long-name-indeed-that-goes-on-and-on-and-on-past-what",
	} {
		if got := Slug(name); got != want {
			t.Errorf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
	if got := Page(12, "Arts & Design"); got != "/feeds/12-arts-design" {
		t.Errorf("Page = %q", got)
	}
	if got := Page(13, "東京"); got != "/feeds/13" {
		t.Errorf("Page = %q", got)
	}
}
