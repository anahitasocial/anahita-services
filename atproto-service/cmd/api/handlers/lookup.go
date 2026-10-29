package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/federation"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Looking somebody up on Bluesky.
//
// A person here types a handle, alice.bsky.social, or pastes the address
// of a profile. The network's reader is asked who that is, and they are
// kept as a person of another server, so that they can be looked at and
// followed like anybody else.

// How many lookups a person may make. Each sends this service out to ask,
// so it is not left without a limit.
var lookupLimit = ratelimit.Limit{Max: 30, Window: 10 * time.Minute}

// RemotePerson is somebody on Bluesky, as the web app is told. The same
// shape as somebody found on the fediverse.
type RemotePerson struct {
	// Their id here.
	ID int64 `json:"id"`
	// Always a Bluesky person's type.
	Type string `json:"type"`
	// Their name.
	Name string `json:"name"`
	// Their handle: alice.bsky.social.
	Alias string `json:"alias"`
	// What they say of themselves.
	Body string `json:"body"`
	// Their address between servers, and where they are kept as from.
	URI          string `json:"uri"`
	OriginDomain string `json:"origin_domain"`
	// Their page on Bluesky.
	URL string `json:"url"`
	// Nobody on Bluesky approves who follows them.
	Locked bool `json:"locked"`
}

// Why somebody could not be found. The same words as for the fediverse,
// which the web app already knows how to say.
const (
	errLookupQuery   = "lookup_unreadable"
	errLookupOurs    = "lookup_is_here"
	errLookupNobody  = "lookup_not_found"
	errLookupFailed  = "lookup_failed"
	errLookupLimit   = "too_many_lookups"
	errLookupBlocked = "lookup_refused"
)

// Lookup finds somebody on Bluesky and keeps them.
func (h *Handlers) Lookup(c echo.Context) error {
	viewer := getViewer(c)

	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "atproto_lookup", fmt.Sprint(viewer.ID), lookupLimit) {
		return refuse(c, http.StatusTooManyRequests, errLookupLimit)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*bsky.Timeout)
	defer cancel()

	person, code := h.lookup(ctx, c.QueryParam("q"))
	if person == nil {
		status := http.StatusNotFound
		switch code {
		case errLookupQuery:
			status = http.StatusBadRequest
		case errLookupFailed:
			status = http.StatusBadGateway
		}
		return refuse(c, status, code)
	}
	return c.JSON(http.StatusOK, person)
}

// LookupFromTheCommandLine does what Lookup does for whoever runs the
// service, with nobody signed in: `app-binary lookup alice.bsky.social`.
// For finding out whether this installation can reach Bluesky at all.
func (h *Handlers) LookupFromTheCommandLine(ctx context.Context, query string) (*RemotePerson, error) {
	person, code := h.lookup(ctx, query)
	if person == nil {
		return nil, errors.New(code)
	}
	return person, nil
}

func (h *Handlers) lookup(ctx context.Context, query string) (*RemotePerson, string) {
	profile, code := h.find(ctx, query)
	if profile == nil {
		return nil, code
	}

	// Somebody here connected that account: it is them, and they are who
	// is found. They are not kept a second time as somebody elsewhere.
	if person := h.connectedPerson(profile.DID); person != nil {
		return &RemotePerson{ID: person.ID, Type: person.Type, Name: person.Name, Alias: person.Alias, Body: person.Body}, ""
	}

	node, err := h.keep(profile)
	if err != nil {
		log.Println("atproto lookup: keeping", profile.DID, ":", err)
		return nil, errLookupFailed
	}
	return &RemotePerson{
		ID:           node.ID,
		Type:         node.Type,
		Name:         node.Name,
		Alias:        node.Alias,
		Body:         node.Body,
		URI:          node.URI,
		OriginDomain: node.OriginDomain,
		URL:          bsky.ProfilePage(profile.DID),
	}, ""
}

// connectedPerson is whoever here connected a Bluesky account, or nil.
func (h *Handlers) connectedPerson(did string) *entities.AnahitaNode {
	if h.Accounts == nil {
		return nil
	}
	personID, err := h.Accounts.WhoConnected(did)
	if err != nil || personID == 0 {
		return nil
	}
	person, err := h.Nodes.Read(personID)
	if err != nil || person == nil || person.ID == 0 {
		return nil
	}
	return person
}

// One person is one profile. A Bluesky account that somebody here
// connected is that person, and is not also kept as somebody of another
// server, with a profile of its own to follow and posts beside theirs.
// What was kept of it before it was connected is removed, and whoever
// followed it here follows nobody by that.

// forgetShadow removes what is kept of a Bluesky account as somebody
// elsewhere, because somebody here connected it.
func (h *Handlers) forgetShadow(did string) {
	kept, err := h.Actors.ReadByDID(did)
	if err != nil || kept == nil {
		return
	}
	if err := h.forget(kept); err != nil {
		log.Printf("what was kept of %s, now connected by somebody here, was not removed: %v", kept.Handle, err)
	}
}

// isOurs says whether a handle is a name of this installation's own: the
// name it goes by, or one under it.
func (h *Handlers) isOurs(handle string) bool {
	domain := strings.ToLower(h.Site.Domain)
	return domain != "" && (handle == domain || strings.HasSuffix(handle, "."+domain))
}

// find reads who a handle, a DID or the address of a profile names. Nil
// and why, when it names nobody who can be kept.
func (h *Handlers) find(ctx context.Context, query string) (*bsky.Profile, string) {
	actor := bsky.ParseActor(query)
	if actor == "" {
		return nil, errLookupQuery
	}
	if h.isOurs(actor) {
		return nil, errLookupOurs
	}
	if h.refused(actor) {
		return nil, errLookupBlocked
	}

	profile, err := h.Bluesky.Profile(ctx, actor)
	switch {
	case errors.Is(err, bsky.ErrNobody):
		return nil, errLookupNobody
	case err != nil:
		log.Println("atproto lookup:", err)
		return nil, errLookupFailed
	}

	// A DID that was asked for has a handle that may be refused, and an
	// account may be refused by itself, whatever it was asked for by.
	if h.refused(strings.ToLower(profile.Handle)) || h.refusedAccount(profile.DID) {
		return nil, errLookupBlocked
	}
	return profile, ""
}

// aliasOf is what somebody on Bluesky is called here: their handle. One
// whose handle no longer leads to them is called by their lasting name,
// written so that it could be no handle.
func aliasOf(profile *bsky.Profile) string {
	handle := strings.ToLower(profile.Handle)
	if handle == "handle.invalid" || !bsky.ValidHandle(handle) {
		return lastingAlias(profile.DID)
	}
	return handle
}

func lastingAlias(did string) string {
	return strings.NewReplacer(":", "-", "%", "-").Replace(strings.ToLower(did)) + ".invalid"
}

// errConnectedHere is an account that is not kept as somebody elsewhere,
// because it is somebody here.
var errConnectedHere = errors.New("that account is somebody here")

// keep makes or brings up to date the node for somebody on Bluesky, and
// what this service alone needs to know of them.
func (h *Handlers) keep(profile *bsky.Profile) (*entities.AnahitaNode, error) {
	if h.connectedPerson(profile.DID) != nil {
		return nil, errConnectedHere
	}

	node := entities.NewAnahitaNode()
	node.Type = constants.NodeActorATProtoPersonEntityType
	node.URI = bsky.ActorURI(profile.DID)
	node.OriginDomain = originDomain
	node.Name = cutRunes(strings.TrimSpace(profile.DisplayName), 255)
	node.Alias = aliasOf(profile)
	node.Body = federation.ToMarkdown(bsky.PlainHTML(profile.Description))
	// Their page on Bluesky: where they are looked at in full.
	node.WebsiteURL = bsky.ProfilePage(profile.DID)

	// What was known of them before, to tell whether a picture changed.
	before, _ := h.Actors.ReadByDID(profile.DID)

	saved, _, err := h.Nodes.UpsertRemote(node)
	// Somebody else kept here goes by that handle: it was theirs once and
	// has been taken over. This one is then called by their lasting name,
	// which nobody else can have.
	if err != nil && strings.Contains(err.Error(), "already known") {
		if fallback := lastingAlias(profile.DID); fallback != node.Alias {
			node.Alias = fallback
			saved, _, err = h.Nodes.UpsertRemote(node)
		}
	}
	if err != nil {
		return nil, err
	}
	if saved == nil || saved.ID == 0 {
		return nil, errors.New("the graph kept nothing")
	}

	avatar, banner := h.pictureAddress(profile.Avatar), h.pictureAddress(profile.Banner)
	err = h.Actors.Save(&repositories.RemoteActor{
		NodeID:    saved.ID,
		DID:       profile.DID,
		Handle:    strings.ToLower(profile.Handle),
		AvatarURL: avatar,
		BannerURL: banner,
	})
	if err != nil {
		return nil, err
	}

	// Their pictures, copied here when new or never copied. Not waited
	// for: who they are does not depend on them.
	if h.Keeper != nil {
		id := saved.ID
		if avatar != "" && (before == nil || before.AvatarURL != avatar || saved.AvatarFilename == "") {
			h.later(func() { h.Keeper.CopyAvatar(id, avatar) })
		}
		if banner != "" && (before == nil || before.BannerURL != banner || saved.CoverFilename == "") {
			h.later(func() { h.Keeper.CopyCover(id, banner) })
		}
	}
	return saved, nil
}

// pictureAddress is where a picture is, if it is somewhere it can be
// fetched from and remembered.
//
// Bluesky's store of pictures sends each in a kind this installation does
// not read unless it is asked for another, which is done by ending the
// address with the kind that is wanted.
func (h *Handlers) pictureAddress(address string) string {
	if !strings.HasPrefix(address, "https://") || len(address) > 500 {
		return ""
	}
	if h.CDN != "" && strings.HasPrefix(address, h.CDN+"/img/") && !strings.Contains(address[len(h.CDN):], "@") {
		address += "@jpeg"
	}
	return address
}

// refresh reads somebody again, because they changed what they say of
// themselves or the handle they go by.
func (h *Handlers) refresh(ctx context.Context, did string) error {
	profile, err := h.Bluesky.Profile(ctx, did)
	if err != nil {
		return err
	}
	if profile.DID != did {
		return fmt.Errorf("asked for %s and was told of %s", did, profile.DID)
	}
	_, err = h.keep(profile)
	return err
}
