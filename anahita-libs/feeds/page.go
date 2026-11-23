package feeds

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// How long a feed's slug may be, in letters: enough to read, short enough
// for a link.
const slugMax = 60

// Slug is a feed's name as its address reads it (roadmap row 176): lower
// case, Latin letters and digits, a dash for anything else; accents taken
// off. "Arts & Design" is "arts-design". A name with nothing Latin in it
// has none, and its feed is at its id alone. Worked out from the name each
// time, so a renamed feed's new address is ready at once, and its old one
// still answers: only the id is read.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			// An accent, taken off the letter before it.
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	slug := b.String()
	// Cut at a word's end: back to the last dash, unless the cut falls on
	// one already.
	if len(slug) > slugMax {
		cut := slug[:slugMax]
		if slug[slugMax] != '-' {
			if dash := strings.LastIndexByte(cut, '-'); dash > 0 {
				cut = cut[:dash]
			}
		}
		slug = cut
	}
	return slug
}

// Page is where a feed is read in the web app: /feeds/12-arts-design, or
// /feeds/12 for a name with no slug.
func Page(id int64, name string) string {
	page := "/feeds/" + strconv.FormatInt(id, 10)
	if slug := Slug(name); slug != "" {
		page += "-" + slug
	}
	return page
}
