package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"activitypub/fetch"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/sessions"

	"github.com/labstack/echo/v4"
)

// Moving one's own account, out and in (roadmap row 124, part 3;
// docs/plans/moving.md).
//
// IN. The new account here names the old ones it also is (alsoKnownAs),
// which the old server reads before it lets the move start; then the old
// server's Move arrives, and is heeded as anybody's is (moving.go).
//
// OUT. The old account here moves to a new one that names it, after the
// person proves who they are again. Its description says where it went
// (movedTo), its followers' servers are sent a Move, its followers here are
// moved at once, and its profile becomes a signpost. auth-service then
// mints its tokens without the scopes that post, like, follow or tag
// (activitypub_accounts.moved_to), until it moves back.
//
// Not within this site: changing one's username does what that would.

const (
	// The most old accounts one account may say it also is.
	aliasesMax = 5
	// How long after a move before another.
	moveAgain = 30 * 24 * time.Hour
)

// AliasesRequest names the old accounts this one also is, by handle or
// address.
type AliasesRequest struct {
	AlsoKnownAs []string `json:"also_known_as" doc:"Handles (ana@old.example) or addresses; at most five. An empty list takes them all away"`
}

// Alias is one old account this one also is.
type Alias struct {
	URI    string `json:"uri" doc:"Its ActivityPub id"`
	Handle string `json:"handle,omitempty" doc:"ana@old.example, when it is kept here"`
}

// Aliases are the old accounts this one also is.
type Aliases struct {
	AlsoKnownAs []Alias `json:"also_known_as"`
}

// AliasRefusal is an old account that could not be found, and why.
type AliasRefusal struct {
	Error string `json:"error" doc:"Why, as for a lookup: lookup_not_found, lookup_is_here, …"`
	Alias string `json:"alias" doc:"The handle or address, as it was asked"`
}

// MoveRequest names the account to move to.
type MoveRequest struct {
	To string `json:"to" doc:"The new account's handle (ana@new.example) or address"`
}

// MoveStatus is whether this account moved, and when it may.
type MoveStatus struct {
	MovedTo *Alias     `json:"moved_to,omitempty" doc:"The account it moved to; absent until it moves"`
	MovedAt *time.Time `json:"moved_at,omitempty" doc:"When it last moved"`
	// When it may move again: thirty days after the last move.
	MayMoveAt *time.Time `json:"may_move_at,omitempty" doc:"When it may move again, while that is still to come"`
}

// ReadAliases is the viewer's own old accounts.
func (h *Handlers) ReadAliases(c echo.Context) error {
	viewer := getViewer(c)
	account, err := h.Accounts.Read(viewer.ID)
	if err != nil {
		log.Println("aliases:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, &Aliases{AlsoKnownAs: h.aliasesOf(account.AlsoKnownAs)})
}

func (h *Handlers) aliasesOf(uris []string) []Alias {
	aliases := make([]Alias, 0, len(uris))
	for _, uri := range uris {
		alias := Alias{URI: uri}
		if node, err := h.Nodes.ReadByURI(uri); err == nil && node != nil {
			alias.Handle = node.Alias
		}
		aliases = append(aliases, alias)
	}
	return aliases
}

// EditAliases names the viewer's old accounts, each found on its server.
func (h *Handlers) EditAliases(c echo.Context) error {
	viewer := getViewer(c)

	req := &AliasesRequest{}
	if err := c.Bind(req); err != nil || req.AlsoKnownAs == nil {
		return refuse(c, http.StatusBadRequest, "also_known_as_required")
	}
	if len(req.AlsoKnownAs) > aliasesMax {
		return refuse(c, http.StatusBadRequest, "too_many_aliases")
	}
	person, account, code := h.movingPerson(viewer.ID)
	if code != "" {
		return refuse(c, http.StatusConflict, code)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*fetch.Timeout)
	defer cancel()

	var uris []string
	for _, asked := range req.AlsoKnownAs {
		asked = strings.TrimSpace(asked)
		if asked == "" {
			continue
		}
		// One named already is not looked up again.
		if slices.Contains(account.AlsoKnownAs, asked) {
			uris = append(uris, asked)
			continue
		}
		actor, why := h.find(ctx, asked)
		if actor == nil {
			return c.JSON(http.StatusNotFound, &AliasRefusal{Error: why, Alias: asked})
		}
		if _, err := h.keep(actor); err != nil {
			log.Println("aliases: keeping", actor.ID, ":", err)
			return refuse(c, http.StatusBadGateway, errLookupFailed)
		}
		if !slices.Contains(uris, actor.ID) {
			uris = append(uris, actor.ID)
		}
	}

	if err := h.Accounts.SetAliases(viewer.ID, uris); err != nil {
		log.Println("aliases:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	// Their followers' servers read them again; any other server reads
	// them when it next asks, as the old one does before a move.
	h.later(func() { h.describedAgain(context.Background(), person, "aliases") })
	return c.JSON(http.StatusOK, &Aliases{AlsoKnownAs: h.aliasesOf(uris)})
}

// ReadMove says whether the viewer's account moved.
func (h *Handlers) ReadMove(c echo.Context) error {
	viewer := getViewer(c)
	account, err := h.Accounts.Read(viewer.ID)
	if err != nil {
		log.Println("move:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, h.statusOf(account, time.Now()))
}

func (h *Handlers) statusOf(account *repositories.Account, now time.Time) *MoveStatus {
	status := &MoveStatus{}
	if account.MovedTo != "" {
		moved := h.aliasesOf([]string{account.MovedTo})[0]
		status.MovedTo = &moved
	}
	if !account.MovedAt.IsZero() {
		at := account.MovedAt.UTC()
		status.MovedAt = &at
		if again := at.Add(moveAgain); again.After(now) {
			status.MayMoveAt = &again
		}
	}
	return status
}

// Move moves the viewer's account to one that names it.
func (h *Handlers) Move(c echo.Context) error {
	viewer := getViewer(c)

	req := &MoveRequest{}
	if err := c.Bind(req); err != nil || strings.TrimSpace(req.To) == "" {
		return refuse(c, http.StatusBadRequest, "to_required")
	}
	person, account, code := h.movingPerson(viewer.ID)
	if code != "" {
		return refuse(c, http.StatusConflict, code)
	}
	if account.MovedTo != "" {
		return refuse(c, http.StatusConflict, "moved_already")
	}
	if !account.MovedAt.IsZero() && time.Since(account.MovedAt) < moveAgain {
		return refuse(c, http.StatusConflict, "moved_lately")
	}

	if h.Session == nil {
		log.Println("move: no session store; refusing")
		return c.NoContent(http.StatusInternalServerError)
	}
	if err := sessions.RequireStepUp(h.Session, viewer.ID); err != nil {
		if errors.Is(err, sessions.ErrStepUpRequired) {
			return refuse(c, http.StatusForbidden, "step_up_required")
		}
		log.Println("move:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*fetch.Timeout)
	defer cancel()

	// Read afresh: whether it names this account is what decides.
	actor, why := h.find(ctx, req.To)
	if actor == nil {
		status := http.StatusNotFound
		if why == errLookupFailed {
			status = http.StatusBadGateway
		}
		return refuse(c, status, why)
	}
	self := h.addresses.Actor(person.UUID)
	if !slices.Contains(actor.AlsoKnownAs, self) {
		return refuse(c, http.StatusConflict, "not_named_there")
	}
	newer, err := h.keep(actor)
	if err != nil {
		log.Println("move: keeping", actor.ID, ":", err)
		return refuse(c, http.StatusBadGateway, errLookupFailed)
	}

	if err := h.movedAway(ctx, person, newer, actor.ID); err != nil {
		log.Println("move:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	log.Printf("moving: %s moved away to %s", person.Alias, newer.Alias)

	account.MovedTo, account.MovedAt = actor.ID, time.Now()
	return c.JSON(http.StatusOK, h.statusOf(account, time.Now()))
}

// ComeBack takes the viewer's move back: the signpost comes down, their
// description no longer says where they went, and the account is whole
// again. Their followers stay with the new account; the new account moves
// to this one to bring them back, which other servers allow only once this
// one no longer says it moved (as Mastodon's "cancel redirect"). The thirty
// days before moving again are still counted from the move.
func (h *Handlers) ComeBack(c echo.Context) error {
	viewer := getViewer(c)
	person, account, code := h.movingPerson(viewer.ID)
	if code != "" {
		return refuse(c, http.StatusConflict, code)
	}
	if account.MovedTo == "" {
		return refuse(c, http.StatusConflict, "not_moved")
	}
	if err := h.Accounts.SetMoved(person.ID, ""); err != nil {
		log.Println("coming back:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	if err := h.Edges.BatchDeleteByFilters(&services.GraphEdgeBatchDeleteByFiltersRequest{
		NodeAID: person.ID, Types: []string{constants.EdgeMovedEntityType},
	}); err != nil {
		log.Println("coming back:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	log.Printf("moving: %s came back from %s", person.Alias, account.MovedTo)
	h.later(func() { h.describedAgain(context.Background(), person, "back:"+account.MovedTo) })

	account.MovedTo = ""
	return c.JSON(http.StatusOK, h.statusOf(account, time.Now()))
}

// movedAway is the move itself, once it is allowed: kept, told, and the
// followers here moved.
func (h *Handlers) movedAway(ctx context.Context, person, newer *entities.AnahitaNode, newURI string) error {
	if err := h.Accounts.SetMoved(person.ID, newURI); err != nil {
		return err
	}
	if err := h.pointTo(person, newer); err != nil {
		return err
	}

	// Their description says where they went, and then each server that
	// follows them is told to move its people.
	self := h.addresses.Actor(person.UUID)
	h.describedAgain(ctx, person, "moved:"+newURI)
	move := ap.NewMove(h.addresses.Activity(newUUID()), self, newURI)
	if err := h.toFollowers(ctx, person, move, "move:"+self+":"+newURI); err != nil {
		return err
	}

	// Who follows them here follows the new account now.
	h.later(func() { h.carryOver(person, newer) })
	return nil
}

// describedAgain sends a person's followers' servers their description as
// it is now, for a change of its own that their profile's updated_at does
// not show.
func (h *Handlers) describedAgain(ctx context.Context, person *entities.AnahitaNode, why string) {
	publicKey, err := h.publicKeyOf(person.ID)
	if err != nil {
		log.Println("described again:", err)
		return
	}
	described := h.actorDescribed(person, publicKey)
	update := ap.NewUpdatePerson(h.addresses.Activity(newUUID()), described)
	key := fmt.Sprintf("profile:%s:%s:%d", person.UUID, why, time.Now().UnixNano())
	if err := h.toFollowers(ctx, person, update, key); err != nil {
		log.Println("described again:", err)
	}
}

// movingPerson is the viewer as somebody who may name old accounts or move:
// a person on the fediverse. Otherwise why not.
func (h *Handlers) movingPerson(personID int64) (*entities.AnahitaNode, *repositories.Account, string) {
	person, err := h.Nodes.Read(personID)
	if err != nil || person == nil || person.ID == 0 || !person.IsPerson() || person.IsRemote() {
		return nil, nil, "not_a_person"
	}
	account, err := h.Accounts.Read(personID)
	if err != nil || account == nil || !account.Enabled {
		return nil, nil, "not_on_fediverse"
	}
	return person, account, ""
}
