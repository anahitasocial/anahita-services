package handlers

import (
	"context"
	"errors"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// For the federation lab only (docs/federation-lab.md).
//
// The web app cannot yet show somebody from another server, and nobody is
// signed in when the lab is driven from a terminal. These do what a person
// here would do by pressing a button, so that what follows from it can be
// tried against another server. main refuses to run them unless
// ACTIVITYPUB_LAB is on, which it cannot be on a live site.

// LabPost makes a public note on somebody's own profile, as posting one
// would, and says what it was numbered.
func (h *Handlers) LabPost(media services.MediaGRPCInterface, alias, text string) (int64, error) {
	person := h.personByAlias(alias)
	if person == nil {
		return 0, errors.New("nobody here is called " + alias)
	}

	viewer := entities.NewViewer()
	viewer.ID = person.ID

	note := entities.NewAnahitaNode()
	note.Type = constants.NodeMediumNoteEntityType
	note.Service = "text-service"
	note.Body = text
	note.OwnerID, note.OwnerType, note.Owner = person.ID, person.Type, person
	note.CreatedBy, note.ModifiedBy = person.ID, person.ID
	note.Enabled = true
	note.CommentStatus = true
	note.Access = constants.AnahitaAccessPublic

	made, err := media.Add(note, viewer)
	if err != nil || made == nil {
		return 0, err
	}
	return made.ID, nil
}

// LabUnpost deletes a note, as its author would.
func (h *Handlers) LabUnpost(media services.MediaGRPCInterface, id int64) error {
	note, err := h.Nodes.Read(id)
	if err != nil || note == nil || note.IsRemote() {
		return errors.New("no such note here")
	}
	viewer := entities.NewViewer()
	viewer.ID = note.CreatedBy
	return media.Delete(id, viewer)
}

// LabFollow makes somebody here follow, or stop following, somebody on
// another server, as they would by pressing the button.
func (h *Handlers) LabFollow(ctx context.Context, alias, handle string, follow bool) error {
	person := h.personByAlias(alias)
	if person == nil {
		return errors.New("nobody here is called " + alias)
	}
	found, err := h.LookupFromTheCommandLine(ctx, handle)
	if err != nil {
		return err
	}
	target, err := h.Nodes.Read(found.ID)
	if err != nil || target == nil {
		return errors.New("the person elsewhere was not kept")
	}

	viewer := entities.NewViewer()
	viewer.ID = person.ID

	if follow {
		add := services.NewSocialgraphFollowRequest().Add
		add.Source, add.Target, add.Viewer = person, target, viewer
		return h.FollowEdges.Add(add)
	}
	remove := services.NewSocialgraphFollowRequest().Delete
	remove.Source, remove.Target, remove.Viewer = person, target, viewer
	return h.FollowEdges.Delete(remove)
}
