package handlers

import (
	"testing"

	"notification/data/entities"
)

func liked(id, by, post int64, read bool) *entities.Notification {
	n := told(id, NotificationTypeVoteup, by, 1)
	n.Object.ID = post
	n.IsRead = read
	return n
}

// WHAT THIS GUARDS (roadmap row 97, part 3): likes of one post are one
// line, at the place of the newest, naming three at most and counting the
// rest, unread while any is; other kinds, and likes of other posts, are
// their own.
func TestLikesOfOnePostAreOne(t *testing.T) {
	items := []*entities.Notification{
		liked(10, 2, 100, false), // Worf, newest
		told(11, NotificationTypeReplyAdd, 3, 1),
		liked(12, 3, 100, true),  // Dax
		liked(13, 4, 200, true),  // Odo, another post
		liked(14, 5, 100, true),  // Kira
		liked(15, 6, 100, true),  // Quark
		liked(16, 2, 100, true),  // Worf again, unliked and liked
		liked(17, 7, 100, false), // Rom
	}
	grouped := groupNotifications(items)

	if len(grouped) != 3 {
		t.Fatalf("%d lines, want 3: post 100's likes, the reply, post 200's like", len(grouped))
	}
	likes := grouped[0]
	if likes.ID != 10 || len(likes.IDs) != 6 {
		t.Errorf("first line %d with %v; want 10, with all six of post 100's", likes.ID, likes.IDs)
	}
	if len(likes.Others) != 2 || likes.Others[0].ID != 3 || likes.Others[1].ID != 5 {
		t.Errorf("others %v; want Dax and Kira named", likes.Others)
	}
	if likes.OthersCount != 2 {
		t.Errorf("%d more, want 2: Quark and Rom", likes.OthersCount)
	}
	if likes.IsRead {
		t.Error("read, though two of them are not")
	}
	if grouped[1].ID != 11 || grouped[2].ID != 13 || len(grouped[2].IDs) != 1 {
		t.Errorf("then %d and %d; want the reply and post 200's like, alone", grouped[1].ID, grouped[2].ID)
	}
}
