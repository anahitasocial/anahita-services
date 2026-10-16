package repositories

import (
	"anahita-libs/data/entities"

	sq "github.com/Masterminds/squirrel"
)

// A photo or video post can be in its author's media library only
// (nodes.library_only): put up from inside the article editor to be shown
// in an article, and not announced. It is read by its address with the
// access any post has, and so the access gates do not know of it; it is
// the LISTINGS that leave it out, each by listedQuery: feeds, a profile's
// lists, search, a hashtag's and a place's posts. Its author lists it in
// the library, and nowhere else.
//
// Not in the gates themselves on purpose. They also answer "may this
// viewer see this post", for a quote, a report, a thread, and a library
// post is to be seen wherever it is shown.

// listedQuery leaves out what is in a media library only.
func listedQuery(sb sq.SelectBuilder) sq.SelectBuilder {
	return sb.Where(sq.Eq{"node.library_only": 0})
}

// listedOrLibrary is a list of posts as it is asked for: the viewer's own
// media library, which is everything of theirs of the kind asked for, what
// is in it only as well as what they posted, as WordPress's media library
// is (the user, 2026-10-16); or any other list, which holds none of what is
// in a library only. A library is nobody's but its author's.
func listedOrLibrary(sb sq.SelectBuilder, library bool, viewer *entities.Viewer) sq.SelectBuilder {
	if !library {
		return listedQuery(sb)
	}
	if viewer == nil || viewer.ID <= 0 {
		return sb.Where("1 = 0")
	}
	return sb.Where(sq.Eq{"node.created_by": viewer.ID})
}
