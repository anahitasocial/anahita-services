package handlers

import (
	"anahita-libs/constants"
	"encoding/json"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
)

// The editable-field whitelists, named rather than inlined.
//
// Edit re-reads the node from the database and copies the caller's values over
// it, so a field missing from applyEditableActorFields keeps its stored value
// and the request's version is discarded — the endpoint returns 200 and saves
// nothing, with no error anywhere and nothing in a log. That is exactly how
// website_url behaved after being added to the column, the proto, the entity
// and the response: correct in five places and dropped here.
//
// Pulled out of the handlers so the list is a reviewable thing with a test
// against it, instead of a loose run of assignments in the middle of a method
// that also loads features, checks subscriptions and publishes an event.

// applyEditableActorFields copies onto an EXISTING node the fields a caller is
// allowed to change.
//
// Deliberately a whitelist. The alternative — replacing the node wholesale
// with the request — would let a caller set follower counts, timestamps, or
// the created_by of somebody else's actor.
func applyEditableActorFields(node *entities.AnahitaNode, req *graphpb.Node) {
	if node == nil || req == nil {
		return
	}

	node.Enabled = req.Enabled
	node.Access = req.Access
	node.Name = req.Name
	node.Body = req.Body
	node.Alias = req.Alias
	node.WebsiteURL = req.WebsiteUrl
	node.Verified = req.Verified
	node.AvatarFilename = req.AvatarFilename
	node.CoverFilename = req.CoverFilename
	node.AllowFollowRequest = req.AllowFollowRequest
	applyEventTimes(node, req)
	node.ShowRepostsOnProfile = req.ShowRepostsOnProfile
	// What this actor's new posts start with, about being downloaded.
	node.AllowDownload = req.AllowDownload
	// Who may quote this person's posts, when a post does not say.
	node.QuotePolicy = req.QuotePolicy
	node.FeaturesRaw = json.RawMessage(req.FeaturesRaw)
	node.ModifiedBy = req.ModifiedBy
	node.PersonPronouns = req.PersonPronouns
	// The language a person usually posts in. Kept as it is when the
	// request carries none: the repository writes it only when it is set.
	node.Language = req.Language
}

// applyEventTimes copies when an event starts and ends, and where it is
// held. Only an event's: on any other actor these are not something anybody
// set, and are left as they were.
func applyEventTimes(node *entities.AnahitaNode, req *graphpb.Node) {
	if node.Type != constants.NodeActorEventEntityType || req.StartsAt == nil || req.EndsAt == nil {
		return
	}

	node.StartsAt = req.StartsAt.AsTime()
	node.EndsAt = req.EndsAt.AsTime()

	// Where it is held, in the node's own address fields, with where that
	// is on a map when whoever made it asked for one. Copied whatever it
	// is, so that either can be taken off again. Nothing searches for an
	// event by them: the searches for what is nearby go by the places
	// tagged on things, not by a node's own coordinates.
	node.GeoLatitude = req.GeoLatitude
	node.GeoLongitude = req.GeoLongitude
	node.GeoAddress = req.GeoAddress
	node.GeoCity = req.GeoCity
	node.GeoStateProvince = req.GeoStateProvince
	node.GeoCountry = req.GeoCountry
	node.GeoPostalcode = req.GeoPostalcode
}

// personTypeIsEditable is deliberately absent from the whitelist above.
//
// person_type is an authority level. Putting it in the editable set would mean
// any caller able to edit an actor could promote it to super-administrator by
// setting a string on the request — the profile edit form included. It is set
// at creation and changed only through a path that checks who is asking.
//
// person_pronouns IS editable: it is display text a person writes about
// themselves, and drives nothing.

// newActorFromRequest builds a NEW node from a create request.
//
// Carries everything applyEditableActorFields does, plus the fields that can
// only be set once: type, service, meta and the creating author. Kept as a
// separate function rather than a flag, because "which fields may be set at
// creation" and "which may be changed afterwards" are different questions and
// answering both from one list is how one of them ends up wrong.
func newActorFromRequest(req *graphpb.Node) *entities.AnahitaNode {
	node := entities.NewAnahitaNode()
	if req == nil {
		return node
	}

	applyEditableActorFields(node, req)

	node.Type = req.Type
	node.Service = req.Service
	// Again, now that it is known to be an event: the first time, above,
	// the new node had no type yet.
	applyEventTimes(node, req)
	node.Meta = json.RawMessage(req.Meta)
	node.CreatedBy = req.CreatedBy
	// Settable once, at creation: signup has to be able to say "registered",
	// and the first-boot seed has to be able to say "super-administrator".
	// Changing it afterwards is not this path's job — see the note above.
	node.PersonType = req.PersonType

	return node
}
