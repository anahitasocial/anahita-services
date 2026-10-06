package handlers

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
	"errors"
	"log"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// placeInThread makes a new post a reply to parent.
//
// The root is the parent's root, or the parent itself when the parent is a
// post at the top level. So every reply in a thread, at any depth, names the
// same root, and deciding who may see one is a single lookup.
//
// A reply has no audience of its own. Its access is left empty, which no
// gate admits by itself: a reply is only ever admitted through its root.
func placeInThread(node *entities.AnahitaNode, parent *entities.AnahitaNode) error {
	if parent == nil || parent.ID <= 0 {
		return errors.New("the post being answered does not exist")
	}

	// Only a post can be answered: not a profile, not a tag, and not one of
	// the old comments, which are not threaded.
	if !strings.HasPrefix(parent.Type, constants.NodeMediumPrefixEntityType) {
		return errors.New("only a post can be replied to")
	}

	// Nothing is added under a reply that has been removed.
	if parent.DeletedAt != nil {
		return errors.New("the post being answered has been removed")
	}

	node.ParentID = parent.ID
	node.ParentType = parent.Type
	node.RootID = parent.RootID
	if node.RootID <= 0 {
		node.RootID = parent.ID
	}
	node.Access = ""

	return nil
}

// Thread returns every reply under a post that the viewer may see, oldest
// first, with which of them are hidden.
//
// Whether the viewer may see the root at all is the caller's question, asked
// before this. The replies are still gated here by the root, so a caller that
// forgot would get nothing rather than everything.
func (h *mediumHandler) Thread(ctx context.Context, req *graphpb.MediumThreadRequest) (*graphpb.MediumThreadResponse, error) {
	if req.GetRootId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "a post is required")
	}

	viewer := entities.NewViewer()
	if req.Viewer != nil {
		viewer.ID = req.Viewer.Id
		viewer.PersonType = req.Viewer.PersonType
		viewer.Username = req.Viewer.Username
		viewer.Email = req.Viewer.Email
	}

	replies, err := h.env.Repos.Replies.Thread(req.RootId, viewer)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	ids := extractNodeIDs(replies)
	if len(ids) > 0 {
		isLikedMap, err := h.env.Repos.LikesRel.IsLikedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		isRepostedMap, err := h.env.Repos.Reposts.IsRepostedMany(viewer.ID, ids)
		if err != nil {
			return nil, err
		}
		for _, reply := range replies {
			reply.IsLikedByViewer = isLikedMap[reply.ID].IsLiked
			reply.IsRepostedByViewer = isRepostedMap[reply.ID]
		}
	}

	hidden, err := h.env.Repos.Replies.HiddenIDs(req.RootId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.MediumThreadResponse{
		Replies:   nodesToGrpc(replies),
		HiddenIds: hidden,
	}, nil
}

// HideReply hides a reply from its thread, or shows it again.
//
// Who may is the caller's to decide, as for every write here.
func (h *mediumHandler) HideReply(ctx context.Context, req *graphpb.ReplyHideRequest) (*graphpb.ReplyHideResponse, error) {
	reply, err := h.env.Repos.Nodes.Read(req.GetReplyId())
	if err != nil {
		return nil, err
	}
	if reply.RootID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "only a reply can be hidden from a thread")
	}

	if err := h.env.Repos.Replies.SetHidden(reply.RootID, reply, req.Hidden, req.ViewerId); err != nil {
		log.Println(err)
		return nil, err
	}

	if err := h.env.Repos.Replies.Recount(reply.RootID, 0); err != nil {
		log.Printf("counting the replies of %d failed: %v", reply.RootID, err)
	}

	return &graphpb.ReplyHideResponse{Success: true}, nil
}

// DetachQuote takes the quoted post out of a note that quotes it, at the
// asking of that post's author. Who may ask is the calling service's to
// check; this records it and counts again.
func (h *mediumHandler) DetachQuote(ctx context.Context, req *graphpb.QuoteDetachRequest) (*graphpb.QuoteDetachResponse, error) {
	note, err := h.env.Repos.Nodes.Read(req.GetNoteId())
	if err != nil {
		return nil, err
	}
	if note.QuoteID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "this note quotes nothing")
	}

	changed, err := h.env.Repos.Quotes.Detach(note.ID, req.GetViewerId())
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if err := h.env.Repos.Quotes.Recount(note.QuoteID); err != nil {
		log.Printf("counting the quotes of %d failed: %v", note.QuoteID, err)
	}

	return &graphpb.QuoteDetachResponse{Success: changed}, nil
}

// SetPinned pins a post to the profile it is on, or takes the pin off. Who
// may is the calling service's to check. A profile has room for
// constants.MaxPinnedPosts; pinning past that moves the pin from the oldest.
func (h *mediumHandler) SetPinned(ctx context.Context, req *graphpb.PinSetRequest) (*graphpb.PinSetResponse, error) {
	node, err := h.env.Repos.Nodes.Read(req.GetNodeId())
	if err != nil {
		return nil, err
	}
	if node.RootID > 0 || !node.IsMedium() {
		return nil, status.Error(codes.InvalidArgument, "only a post can be pinned")
	}

	if req.GetPinned() {
		err = h.env.Repos.Pins.Pin(node.ID, node.OwnerID, req.GetViewerId())
	} else {
		err = h.env.Repos.Pins.Unpin(node.ID, req.GetViewerId())
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &graphpb.PinSetResponse{Success: true}, nil
}
