package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/remote"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// Looking somebody up on another server.
//
// A person here types a handle, @alice@mastodon.example, or pastes the
// address of a profile. This asks that server who it is, and keeps them as
// a person of another server, so that they can be looked at and followed
// like anybody else.

// How many lookups a person may make. Each sends this service to a server
// somebody named, so it is not left without a limit.
var lookupLimit = ratelimit.Limit{Max: 30, Window: 10 * time.Minute}

// RemotePerson is somebody on another server, as the web app is told.
type RemotePerson struct {
	// Their id here.
	ID int64 `json:"id"`
	// Always the type of a person on the fediverse.
	Type string `json:"type"`
	// Their name.
	Name string `json:"name"`
	// Their handle: alice@mastodon.example.
	Alias string `json:"alias"`
	// What they say of themselves.
	Body string `json:"body"`
	// Their address between servers, and the server it is on.
	URI          string `json:"uri"`
	OriginDomain string `json:"origin_domain"`
	// Their page on their own server.
	URL string `json:"url"`
	// Whether a follow of theirs waits to be approved.
	Locked bool `json:"locked"`
}

// Why somebody could not be found.
const (
	errLookupQuery  = "lookup_unreadable"
	errLookupOurs   = "lookup_is_here"
	errLookupNobody = "lookup_not_found"
	errLookupKind   = "lookup_not_a_person"
	errLookupFailed = "lookup_failed"
	errLookupLimit  = "too_many_lookups"
	// The server asked about is one this installation refuses.
	errLookupBlocked = "lookup_refused"
)

// Lookup finds somebody on another server and keeps them.
func (h *Handlers) Lookup(c echo.Context) error {
	viewer := getViewer(c)

	if ok, reason := h.siteFederates(); !ok {
		return refuse(c, http.StatusConflict, reason)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "activitypub_lookup", fmt.Sprint(viewer.ID), lookupLimit) {
		return refuse(c, http.StatusTooManyRequests, errLookupLimit)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*fetch.Timeout)
	defer cancel()

	actor, code := h.find(ctx, c.QueryParam("q"))
	if actor == nil {
		status := http.StatusNotFound
		switch code {
		case errLookupQuery:
			status = http.StatusBadRequest
		case errLookupFailed:
			status = http.StatusBadGateway
		}
		return refuse(c, status, code)
	}

	person, err := h.keepAs(actor)
	if err != nil {
		log.Println("activitypub lookup: keeping", actor.ID, ":", err)
		return refuse(c, http.StatusBadGateway, errLookupFailed)
	}

	return c.JSON(http.StatusOK, person)
}

// LookupFromTheCommandLine does what Lookup does for whoever runs the
// service, with nobody signed in: `app-binary lookup @alice@example.net`.
// For finding out whether this installation and another can reach and
// understand each other, which is the first thing to know when they cannot.
func (h *Handlers) LookupFromTheCommandLine(ctx context.Context, query string) (*RemotePerson, error) {
	if ok, reason := h.siteFederates(); !ok {
		return nil, errors.New(reason)
	}
	actor, code := h.find(ctx, query)
	if actor == nil {
		return nil, errors.New(code)
	}
	return h.keepAs(actor)
}

func (h *Handlers) keepAs(actor *remote.Actor) (*RemotePerson, error) {
	node, err := h.keep(actor)
	if err != nil {
		return nil, err
	}
	return &RemotePerson{
		ID:           node.ID,
		Type:         node.Type,
		Name:         node.Name,
		Alias:        node.Alias,
		Body:         node.Body,
		URI:          node.URI,
		OriginDomain: node.OriginDomain,
		URL:          actor.Page,
		Locked:       actor.Locked,
	}, nil
}

// find reads who a handle or an address names, on another server. Nil and
// why, when it names nobody who can be kept.
func (h *Handlers) find(ctx context.Context, query string) (*remote.Actor, string) {
	query = strings.TrimSpace(query)

	var (
		actor *remote.Actor
		err   error
	)

	switch {
	case strings.HasPrefix(query, "https://"):
		parsed, parseErr := url.Parse(query)
		if parseErr != nil || parsed.Host == "" {
			return nil, errLookupQuery
		}
		if h.Site.IsOurs(query) {
			return nil, errLookupOurs
		}
		if h.Blocks != nil && h.Blocks.IsBlocked(parsed.Host) {
			return nil, errLookupBlocked
		}
		actor, err = remote.Fetch(ctx, h.Fetcher, query, h.Site.IsOurs)

	default:
		// @alice@mastodon.example, with or without the first @.
		mentions := entities.ParseMentions("@" + strings.TrimPrefix(query, "@"))
		if len(mentions) != 1 || mentions[0].Host == "" || mentions[0].End != len(strings.TrimPrefix(query, "@"))+1 {
			return nil, errLookupQuery
		}
		if mentions[0].IsLocal(h.Site.Domain) {
			return nil, errLookupOurs
		}
		if h.Blocks != nil && h.Blocks.IsBlocked(mentions[0].Host) {
			return nil, errLookupBlocked
		}
		actor, err = remote.Lookup(ctx, h.Fetcher, mentions[0].User, mentions[0].Host, h.Site.IsOurs)
	}

	// A handle on one server can lead to somebody on another.
	if err == nil && h.Blocks != nil && h.Blocks.IsBlocked(actor.Domain) {
		return nil, errLookupBlocked
	}

	switch {
	case err == nil:
		return actor, ""
	case errors.Is(err, fetch.ErrGone), errors.Is(err, remote.ErrNoSuchOne):
		return nil, errLookupNobody
	case errors.Is(err, remote.ErrNotAPerson):
		return nil, errLookupKind
	case errors.Is(err, fetch.ErrNotAllowed), errors.Is(err, remote.ErrNotTheirs):
		// Somewhere this does not go, or a server saying what it may not.
		// To the person asking, nobody is there.
		log.Println("activitypub lookup:", err)
		return nil, errLookupNobody
	}

	log.Println("activitypub lookup:", err)
	return nil, errLookupFailed
}

// keep makes or brings up to date the node for somebody on another server,
// and what this service alone needs to know of them.
func (h *Handlers) keep(actor *remote.Actor) (*entities.AnahitaNode, error) {
	node := entities.NewAnahitaNode()
	node.Type = constants.NodeActorActivityPubPersonEntityType
	node.URI = actor.ID
	node.OriginDomain = actor.Domain
	node.Name = actor.Name
	node.Alias = actor.Handle
	node.Body = actor.Bio
	// Their page on their own server: where they are looked at in full.
	node.WebsiteURL = actor.Page

	// What was known of them before, to tell whether their picture changed.
	before, _ := h.Actors.ReadByURI(actor.ID)

	saved, _, err := h.Nodes.UpsertRemote(node)
	// Somebody else kept here goes by that handle: it was theirs once and
	// has been taken over, or it is claimed by two. This one is then called
	// by the server they are on, which nobody else can be.
	if err != nil && strings.Contains(err.Error(), "already known") {
		fallback := strings.ToLower(actor.Username) + "@" + actor.Domain
		if fallback != node.Alias {
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

	err = h.Actors.Save(&repositories.RemoteActor{
		NodeID:      saved.ID,
		URI:         actor.ID,
		Inbox:       actor.Inbox,
		SharedInbox: actor.SharedInbox,
		KeyID:       actor.KeyID,
		PublicKey:   actor.PublicKeyPEM,
		IconURL:     actor.Icon,
		HeaderURL:   actor.Header,
		Locked:      actor.Locked,
	})
	if err != nil {
		return nil, err
	}

	// Their picture, copied here when it is new or was never copied. Not
	// waited for: who they are does not depend on it.
	if actor.Icon != "" && (before == nil || before.IconURL != actor.Icon || saved.AvatarFilename == "") {
		id, icon := saved.ID, actor.Icon
		h.later(func() { h.copyAvatar(id, icon) })
	}
	if actor.Header != "" && (before == nil || before.HeaderURL != actor.Header || saved.CoverFilename == "") {
		id, header := saved.ID, actor.Header
		h.later(func() { h.copyCover(id, header) })
	}
	return saved, nil
}

// later runs what need not hold up an answer.
func (h *Handlers) later(work func()) {
	if h.Later != nil {
		h.Later(work)
		return
	}
	work()
}
