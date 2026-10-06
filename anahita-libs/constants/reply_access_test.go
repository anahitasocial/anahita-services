package constants

import "testing"

func TestNormalizeReplyAccess(t *testing.T) {
	cases := []struct {
		raw   string
		want  string
		valid bool
	}{
		{"", "", true},
		{"anyone", "anyone", true},
		{" Nobody ", "nobody", true},
		{"followers", "followers", true},
		// Stored in one order whatever order they arrive in, and once each.
		{"mentioned, followers", "followers,mentioned", true},
		{"following,following,followers", "followers,following", true},
		{"followers,following,mentioned", "followers,following,mentioned", true},

		// The two that stand alone do not go beside a group.
		{"anyone,followers", "", false},
		{"followers,nobody", "", false},
		// Not a group at all, including the names of audiences.
		{"friends", "", false},
		{"public", "", false},
		{"followers,", "", false},
	}

	for _, c := range cases {
		got, valid := NormalizeReplyAccess(c.raw)
		if got != c.want || valid != c.valid {
			t.Errorf("NormalizeReplyAccess(%q) = %q, %v; want %q, %v", c.raw, got, valid, c.want, c.valid)
		}
	}
}

func TestReplyAccessHas(t *testing.T) {
	if !ReplyAccessHas("followers,mentioned", ReplyAccessMentioned) {
		t.Error("mentioned is in followers,mentioned")
	}
	if ReplyAccessHas("followers,mentioned", ReplyAccessFollowing) {
		t.Error("following is not in followers,mentioned")
	}
	if ReplyAccessHas("", ReplyAccessFollowers) || ReplyAccessHas("anyone", ReplyAccessFollowers) {
		t.Error("a group was found where none was chosen")
	}
}
