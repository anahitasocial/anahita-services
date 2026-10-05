// Package language names the language a post is written in.
//
// A post carries one language tag, chosen by its author. It is what a screen
// reader uses to pronounce the post, what other fediverse servers file it
// under (ActivityPub's contentMap, Bluesky's langs), and what a translate
// button or a language filter would later go by.
//
// The author chooses, and nothing here guesses. A post whose client sent no
// language has none, which is different from a wrong one: other servers show
// an untagged post to everybody, while a mistagged one is hidden from the
// people who could read it.
//
// The tags are BCP 47, kept simple: a language, and optionally a region.
//
//	en      English
//	fr-CA   French as written in Canada
//	fa      Persian
//	und     undetermined: the author said they do not want to say
//
// The language part is checked against ISO 639-1, the two-letter codes, so a
// typo or an invented code is refused. Three-letter codes are accepted as
// they come: there are thousands, and a language with no two-letter code is
// exactly the one a short list would wrongly leave out.
package language

import (
	"regexp"
	"strings"
)

// Undetermined is the tag for "no particular language", or "not saying".
const Undetermined = "und"

// MaxLength is the longest tag accepted: a language and a region.
const MaxLength = 12

var tagShape = regexp.MustCompile(`^([a-zA-Z]{2,3})(?:[-_]([a-zA-Z]{2}|[0-9]{3}))?$`)

// Normalize returns a tag in its usual spelling, the language in lower case
// and the region in upper case ("FR-ca" becomes "fr-CA"), and whether it is
// one this package accepts.
//
// An empty tag is not valid. A caller for which "not given" is acceptable
// checks for that first.
func Normalize(tag string) (string, bool) {
	tag = strings.TrimSpace(tag)
	if len(tag) > MaxLength {
		return "", false
	}

	parts := tagShape.FindStringSubmatch(tag)
	if parts == nil {
		return "", false
	}

	primary := strings.ToLower(parts[1])
	if len(primary) == 2 && !iso6391[primary] {
		return "", false
	}

	if parts[2] == "" {
		return primary, true
	}

	return primary + "-" + strings.ToUpper(parts[2]), true
}

// Primary returns the language part of a tag: "fr" for "fr-CA".
func Primary(tag string) string {
	if cut := strings.IndexAny(tag, "-_"); cut != -1 {
		tag = tag[:cut]
	}
	return strings.ToLower(tag)
}

// iso6391 is every two-letter language code.
var iso6391 = func() map[string]bool {
	const codes = "aa ab ae af ak am an ar as av ay az ba be bg bh bi bm bn bo br bs ca ce ch co cr cs cu cv cy " +
		"da de dv dz ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr ht hu hy hz " +
		"ia id ie ig ii ik io is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg li ln lo lt lu lv " +
		"mg mh mi mk ml mn mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi pl ps pt qu " +
		"rm rn ro ru rw sa sc sd se sg si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl tn to tr ts tt tw ty " +
		"ug uk ur uz ve vi vo wa wo xh yi yo za zh zu"

	known := map[string]bool{}
	for _, code := range strings.Fields(codes) {
		known[code] = true
	}
	return known
}()
