package handlers

import (
	"encoding/json"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// What a caller may set on a post, in one place for creating and one for
// editing, as actor_fields.go is for actors.
//
// Both copy a list of fields, so a field left off the list is dropped
// without a word: the request succeeds and the column keeps what it had. A
// post's language did exactly that. It was accepted by the services, carried
// in the request, written by the repository, and never copied here, so every
// post was saved with none. The tests beside this file walk the lists, so
// the next field is covered by adding its name there.

// applyEditableMediumFields copies onto an existing post what an edit may
// change.
func applyEditableMediumFields(node *entities.AnahitaNode, req *graphpb.Node) {
	if node == nil || req == nil {
		return
	}

	node.Enabled = req.Enabled
	node.Access = req.Access
	node.Name = req.Name
	node.Alias = req.Alias
	node.Body = req.Body
	node.Excerpt = req.Excerpt
	node.Filename = req.Filename
	node.CoverFilename = req.CoverFilename
	node.Meta = json.RawMessage(req.Meta)
	node.ModifiedBy = req.ModifiedBy
	node.CommentStatus = req.CommentStatus
	node.ReplyAccess = req.ReplyAccess
	node.QuotePolicy = req.QuotePolicy
	// Whether it may be downloaded. An edit of anything else carries what
	// was read, and leaves it as it was.
	node.AllowDownload = req.AllowDownload
	node.CommentCount = req.CommentCount
	// Kept as it is when the request carries none: the repository writes
	// it only when it is set.
	node.Language = req.Language
}

// newMediumFromRequest builds a new post: what an edit may change, and what
// is set once.
func newMediumFromRequest(req *graphpb.Node) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	if req == nil {
		return node
	}

	applyEditableMediumFields(node, req)

	node.OwnerID = req.OwnerId
	node.OwnerType = req.OwnerType
	node.Type = req.Type
	node.Service = req.Service
	node.Mimetype = req.Mimetype
	node.CreatedBy = req.CreatedBy
	// What it quotes, which is said once and never changed.
	node.QuoteID = req.QuoteId
	// In its author's media library only, which is said once too: a post
	// once announced cannot be taken back into the library.
	node.LibraryOnly = req.LibraryOnly

	return node
}
