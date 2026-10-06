package handlers

import (
	"net/http"

	"anahita-libs/constants"
	"anahita-libs/data/entities"

	"github.com/labstack/echo/v4"
)

// ErrReplyAccessInvalid is the error a request gets when the value it sends
// for who may reply is not one.
const ErrReplyAccessInvalid = "reply_access_invalid"

// RequestReplyAccess reads who may reply from a request.
//
// Not given is fine, and comes back as "" and true. A value that is given
// comes back in its stored form. One that is not a value comes back false,
// for the handler to refuse with RefuseReplyAccess.
func RequestReplyAccess(raw string) (string, bool) {
	return constants.NormalizeReplyAccess(raw)
}

// RefuseReplyAccess answers a request whose value for who may reply is not
// one: 400, saying which field.
func RefuseReplyAccess(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error":       ErrReplyAccessInvalid,
		"description": "reply_access must be \"anyone\", \"nobody\", or any of \"followers\", \"following\" and \"mentioned\" joined by commas",
	})
}

// SetReplyAccess sets who may reply to a post, and the older switch for
// replies with it: off for "nobody" and on for everything else. The two are
// never left saying different things.
func SetReplyAccess(post *entities.AnahitaNode, access string) {
	post.ReplyAccess = access
	post.CommentStatus = access != constants.ReplyAccessNobody
}

// SetRepliesOpen is the older switch: on or off. Turning it off is "nobody".
// Turning it on leaves a choice of groups as it was, and makes "nobody"
// "anyone".
func SetRepliesOpen(post *entities.AnahitaNode, open bool) {
	post.CommentStatus = open

	switch {
	case !open:
		post.ReplyAccess = constants.ReplyAccessNobody
	case post.ReplyAccess == constants.ReplyAccessNobody || post.ReplyAccess == "":
		post.ReplyAccess = constants.ReplyAccessAnyone
	}
}
