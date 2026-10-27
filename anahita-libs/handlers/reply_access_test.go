package handlers

import (
	"testing"

	"anahita-libs/data/entities"
)

// The value and the older switch are changed together, whichever of the two
// a request changes.
func TestWhoMayReplyAndTheSwitchStayInStep(t *testing.T) {
	post := entities.NewAnahitaNode()

	SetReplyAccess(post, "nobody")
	if post.CommentStatus || post.ReplyAccess != "nobody" {
		t.Errorf("nobody: switch %v, value %q", post.CommentStatus, post.ReplyAccess)
	}

	SetReplyAccess(post, "followers,mentioned")
	if !post.CommentStatus || post.ReplyAccess != "followers,mentioned" {
		t.Errorf("groups: switch %v, value %q", post.CommentStatus, post.ReplyAccess)
	}

	// Off and on again by the switch: the groups are gone, because off
	// was "nobody", and on from "nobody" is "anyone".
	SetRepliesOpen(post, false)
	if post.CommentStatus || post.ReplyAccess != "nobody" {
		t.Errorf("switched off: switch %v, value %q", post.CommentStatus, post.ReplyAccess)
	}

	SetRepliesOpen(post, true)
	if !post.CommentStatus || post.ReplyAccess != "anyone" {
		t.Errorf("switched on: switch %v, value %q", post.CommentStatus, post.ReplyAccess)
	}

	// On while already on leaves a choice of groups alone.
	SetReplyAccess(post, "following")
	SetRepliesOpen(post, true)
	if post.ReplyAccess != "following" {
		t.Errorf("switched on while on: value %q, want following", post.ReplyAccess)
	}
}
