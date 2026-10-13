package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// Leaving the fediverse, and changing what one is known as on it.
//
// Somebody leaves when they turn it off, and also when they stop being
// somebody who may be on it: their profile is no longer open to everybody,
// they are disabled, archived or deleted. Either way every server that
// knows them is told to forget them, with a Delete of the person, which is
// the one message the fediverse has for it.
//
// LEAVING IS FOR GOOD, AS FAR AS OTHER SERVERS GO. A server that is told
// somebody is deleted removes them, their posts and their followers, and
// many will not take the same person back. Turning it on again makes them
// findable again here, and whether another server will know them again is
// that server's choice. The web app says so before it is turned off.

// leave tells every server that knows a person to forget them, and undoes
// here what crossed between servers: who they follow elsewhere, who
// follows them from elsewhere, and the record of what they sent.
func (h *Handlers) leave(ctx context.Context, person *entities.AnahitaNode) error {
	if person == nil || person.ID == 0 || person.UUID == "" {
		return nil
	}

	inboxes, err := h.Follows.KnownInboxes(person.ID)
	if err != nil {
		return err
	}
	follows, err := h.Follows.AllFor(person.ID)
	if err != nil {
		return err
	}

	actor := h.addresses.Actor(person.UUID)
	gone := ap.NewDeletePerson(h.addresses.Activity(newUUID()), actor)
	for _, inbox := range inboxes {
		if err := h.send(ctx, person.ID, inbox, gone, "leave:"+actor); err != nil {
			return err
		}
	}

	// Forgotten before the follows are taken away, so that their going is
	// not taken for unfollowing and answered with an Undo to a server that
	// has just been told the person is gone.
	if err := h.Follows.DeleteAllFor(person.ID); err != nil {
		return err
	}
	if err := h.Objects.EndAll(person.ID); err != nil {
		return err
	}

	viewer := entities.NewViewer()
	viewer.ID = person.ID
	for _, follow := range follows {
		other, err := h.Nodes.Read(follow.RemoteID)
		if err != nil || other == nil || other.ID == 0 {
			continue
		}
		remove := services.NewSocialgraphFollowRequest().Delete
		remove.Viewer = viewer
		if follow.Direction == repositories.FollowOut {
			remove.Source, remove.Target = person, other
		} else {
			remove.Source, remove.Target = other, person
		}
		if err := h.FollowEdges.Delete(remove); err != nil {
			log.Printf("leaving: the follow between %d and %d stays: %v", person.ID, other.ID, err)
		}
	}

	return h.Accounts.Set(person.ID, false)
}

// profileChanged is something having happened to a person here: their
// profile edited, or their account disabled, archived, deleted or purged.
// Other servers are told what follows from it.
func (h *Handlers) profileChanged(ctx context.Context, personID int64) error {
	account, err := h.Accounts.Read(personID)
	if err != nil || account == nil || !account.Enabled {
		// Not on the fediverse: nobody to tell.
		return err
	}

	person, err := h.Nodes.Read(personID)
	if err != nil {
		return err
	}
	if person == nil || person.ID == 0 || person.Type != constants.NodeActorPersonEntityType {
		return nil
	}

	// No longer somebody who may be on it: they leave.
	if !h.mayFederate(person) {
		log.Printf("%s may no longer be on the fediverse: other servers are told to forget them", person.Alias)
		return h.leave(ctx, person)
	}

	// Still on it: their followers' servers are sent them as they are now.
	publicKey, err := h.publicKeyOf(person.ID)
	if err != nil {
		return err
	}
	described := ap.NewActor(h.addresses, person, publicKey)
	changed := person.UpdatedAt.UTC().Format(time.RFC3339)
	if err := h.toFollowers(ctx, person,
		ap.NewUpdatePerson(h.addresses.Activity(newUUID()), described),
		fmt.Sprintf("profile:%s:%s", person.UUID, changed)); err != nil {
		return err
	}
	return h.Accounts.Told(person.ID, person.UpdatedAt)
}
