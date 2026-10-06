package services

import (
	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"context"
)

// Thread returns every reply under a post that the viewer may see.
func (srv *MediaGRPC) Thread(rootID int64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.Thread(context.Background(), &graphpb.MediumThreadRequest{
		RootId: rootID,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}

	hidden := map[int64]bool{}
	for _, id := range resp.GetHiddenIds() {
		hidden[id] = true
	}

	replies := make([]*entities.AnahitaNode, 0, len(resp.GetReplies()))
	for _, node := range resp.GetReplies() {
		reply := grpcToGraphNode(node)
		reply.Hidden = hidden[reply.ID]
		replies = append(replies, reply)
	}

	return replies, nil
}

// HideReply hides a reply from its thread, or shows it again.
func (srv *MediaGRPC) HideReply(replyID int64, hidden bool, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.HideReply(context.Background(), &graphpb.ReplyHideRequest{
		ReplyId:  replyID,
		Hidden:   hidden,
		ViewerId: viewer.ID,
	})
	return err
}

// DetachQuote takes the quoted post out of a note that quotes it. The note
// stays, saying that the post was removed by its author.
func (srv *MediaGRPC) DetachQuote(noteID int64, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.DetachQuote(context.Background(), &graphpb.QuoteDetachRequest{
		NoteId:   noteID,
		ViewerId: viewer.ID,
	})
	return err
}

// Quotes returns the notes that quote a post and that the viewer may see,
// newest first.
func (srv *MediaGRPC) Quotes(quotedID int64, viewer *entities.Viewer) ([]*entities.AnahitaNode, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, err
	}

	resp, err := client.ListQuotes(context.Background(), &graphpb.QuotesListRequest{
		QuotedId: quotedID,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, err
	}

	quotes := make([]*entities.AnahitaNode, 0, len(resp.GetQuotes()))
	for _, node := range resp.GetQuotes() {
		quotes = append(quotes, grpcToGraphNode(node))
	}

	return quotes, nil
}

// SetSaved saves a post for the viewer, or takes it out of what they saved.
func (srv *MediaGRPC) SetSaved(nodeID int64, saved bool, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.SetSaved(context.Background(), &graphpb.SavedSetRequest{
		NodeId:   nodeID,
		Saved:    saved,
		ViewerId: viewer.ID,
	})
	return err
}

// Saved returns what the viewer saved, most recently saved first, as they
// may see it now, and how many there are.
func (srv *MediaGRPC) Saved(limit uint64, offset uint64, viewer *entities.Viewer) ([]*entities.AnahitaNode, uint64, error) {
	client, err := srv.getClient()
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.ListSaved(context.Background(), &graphpb.SavedListRequest{
		Limit:  limit,
		Offset: offset,
		Viewer: &graphpb.Viewer{
			Id:         viewer.ID,
			PersonType: viewer.PersonType,
			Email:      viewer.Email,
			Username:   viewer.Username,
		},
	})
	if err != nil {
		return nil, 0, err
	}

	nodes := make([]*entities.AnahitaNode, 0, len(resp.GetNodes()))
	for _, node := range resp.GetNodes() {
		nodes = append(nodes, grpcToGraphNode(node))
	}

	return nodes, resp.GetTotal(), nil
}

// SetPinned pins a post to the profile it is on, or takes the pin off. A
// profile has one pin: pinning moves it from the post that had it.
func (srv *MediaGRPC) SetPinned(nodeID int64, pinned bool, viewer *entities.Viewer) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	_, err = client.SetPinned(context.Background(), &graphpb.PinSetRequest{
		NodeId:   nodeID,
		Pinned:   pinned,
		ViewerId: viewer.ID,
	})
	return err
}

// VisibleInThread leaves out of a thread what most people are not shown:
// the replies that have been hidden from it, and everything said under
// them. Whoever looks after the thread is given the whole of it, with the
// hidden ones marked, so they can show one again.
//
// Replies arrive oldest first, so a reply always comes after what it
// answers, and one pass is enough.
func VisibleInThread(replies []*entities.AnahitaNode, seesHidden bool) []*entities.AnahitaNode {
	gone := map[int64]bool{}
	visible := make([]*entities.AnahitaNode, 0, len(replies))

	for _, reply := range replies {
		if reply.Hidden || gone[reply.ParentID] {
			gone[reply.ID] = true
			// Marked hidden all the way down, so whoever is shown them
			// sees that the whole branch is out of the thread.
			reply.Hidden = true
		}

		if gone[reply.ID] && !seesHidden {
			continue
		}

		visible = append(visible, reply)
	}

	return visible
}
