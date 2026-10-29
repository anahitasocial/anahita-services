package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/event"
	"anahita-libs/federation"
	"anahita-libs/responses"

	"github.com/labstack/echo/v4"
)

// The accounts on other networks this installation will have nothing to
// do with, one by one (docs/plans/done/refused-accounts.md): a fediverse account
// by its address, a Bluesky account by its DID. Refused by administrators,
// as moderating one account is theirs; a whole server stays with super
// administrators (blocks.go).
//
// Refused, an account is not listened to, is sent nothing, cannot be looked
// up or followed, and is removed with what is here of it: at once when it
// is on the fediverse, which this service keeps; within a minute when it is
// on Bluesky, which atproto-service keeps and looks for on the list.

// errAccountRefused is an account that is refused, met where it would be
// kept.
var errAccountRefused = errors.New("the account is refused")

// accountRefused says whether a fediverse account is refused by itself.
func (h *Handlers) accountRefused(address string) bool {
	return h.RefusedAccounts != nil && address != "" &&
		h.RefusedAccounts.IsRefused(federation.NetworkActivityPub, address)
}

// RefuseAccountRequest names the account to refuse: one kept here, by its
// profile's id, or any, by its handle or address.
type RefuseAccountRequest struct {
	// The profile kept here.
	ActorID int64 `json:"actor_id"`
	// Or a handle or an address: `@user@server`, an actor's address, a
	// Bluesky handle or a DID.
	Address string `json:"address"`
	// Why it is refused, for the administrators who come after.
	Reason string `json:"reason"`
}

// BrowseRefusedAccounts lists the accounts that are refused, newest first,
// a page at a time: `q` narrows it to the names, handles and reasons it is
// found in.
func (h *Handlers) BrowseRefusedAccounts(c echo.Context) error {
	if !getViewer(c).IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit < 1 || limit > blocksPageMax {
		limit = blocksPage
	}
	offset, _ := strconv.Atoi(c.QueryParam("start"))
	if offset < 0 {
		offset = 0
	}

	accounts, total, err := h.RefusedAccounts.Browse(c.QueryParam("q"), limit, offset)
	if err != nil {
		log.Println("refused accounts:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.JSON(http.StatusOK, responses.List[*federation.RefusedAccount]{
		Data: accounts,
		Pagination: responses.Pagination{
			Total:  uint64(total),
			Limit:  uint64(limit),
			Offset: uint64(offset),
		},
	})
}

// RefuseAccount refuses an account, and removes what is here of it.
func (h *Handlers) RefuseAccount(c echo.Context) error {
	viewer := getViewer(c)
	if !viewer.IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}

	req := &RefuseAccountRequest{}
	if err := c.Bind(req); err != nil || (req.ActorID <= 0 && strings.TrimSpace(req.Address) == "") {
		return refuse(c, http.StatusBadRequest, "account_required")
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
	defer cancel()

	var (
		account *federation.RefusedAccount
		code    string
		status  int
	)
	if req.ActorID > 0 {
		account, status, code = h.refusedByProfile(req.ActorID)
	} else {
		account, status, code = h.refusedByAddress(ctx, req.Address)
	}
	if account == nil {
		return refuse(c, status, code)
	}
	account.Reason, account.CreatedBy = reason, viewer.ID

	added, err := h.RefusedAccounts.Add(account)
	if errors.Is(err, federation.ErrAlreadyRefused) {
		return refuse(c, http.StatusConflict, "already_refused")
	}
	if err != nil {
		log.Println("refused accounts:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}

	// What is here of it goes. On the fediverse at once; on Bluesky,
	// atproto-service finds it on the list within a minute.
	if added.Network == federation.NetworkActivityPub {
		if kept, err := h.Actors.ReadByURI(added.Identifier); err == nil && kept != nil {
			if err := h.forget(kept); err != nil {
				log.Printf("refused account %s was not removed: %v", added.Identifier, err)
			}
		}
	}
	return c.JSON(http.StatusCreated, added)
}

// refusedByProfile is the account a profile kept here is: its address, or
// its DID. Never a profile made here.
func (h *Handlers) refusedByProfile(id int64) (*federation.RefusedAccount, int, string) {
	node, err := h.Nodes.Read(id)
	if err != nil || node == nil || node.ID == 0 {
		return nil, http.StatusNotFound, "account_not_found"
	}

	account := &federation.RefusedAccount{Handle: node.Alias, Name: node.Name}
	switch node.Type {
	case constants.NodeActorActivityPubPersonEntityType:
		kept, err := h.Actors.ReadByNode(node.ID)
		if err != nil || kept == nil {
			return nil, http.StatusNotFound, "account_not_found"
		}
		account.Network, account.Identifier = federation.NetworkActivityPub, kept.URI
	case constants.NodeActorATProtoPersonEntityType:
		did := strings.TrimPrefix(node.URI, "at://")
		if !strings.HasPrefix(did, "did:") {
			return nil, http.StatusNotFound, "account_not_found"
		}
		account.Network, account.Identifier = federation.NetworkATProto, did
	default:
		return nil, http.StatusConflict, "is_local"
	}
	return account, 0, ""
}

// A Bluesky handle as it is typed: a name with a dot in it and nothing else.
var blueskyHandle = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)

// refusedByAddress is the account a handle or an address names, kept here
// or not: a DID as it is, a Bluesky handle by the DID it stands for, and
// anything else looked up on the fediverse.
func (h *Handlers) refusedByAddress(ctx context.Context, address string) (*federation.RefusedAccount, int, string) {
	address = strings.TrimSpace(address)
	lower := strings.ToLower(strings.TrimPrefix(address, "@"))

	switch {
	case strings.HasPrefix(lower, "did:"):
		return &federation.RefusedAccount{Network: federation.NetworkATProto, Identifier: lower}, 0, ""

	case blueskyHandle.MatchString(lower) && !strings.Contains(address, "@"):
		did, err := h.resolveBlueskyHandle(ctx, lower)
		if err != nil || did == "" {
			return nil, http.StatusNotFound, "account_not_found"
		}
		return &federation.RefusedAccount{Network: federation.NetworkATProto, Identifier: did, Handle: lower}, 0, ""
	}

	actor, why := h.find(ctx, address)
	switch why {
	case "":
	case errLookupOurs:
		return nil, http.StatusConflict, "is_local"
	case errLookupBlocked:
		// Its server, or itself, is refused already.
		return nil, http.StatusConflict, "already_refused"
	case errLookupQuery:
		return nil, http.StatusBadRequest, "account_unreadable"
	default:
		return nil, http.StatusNotFound, "account_not_found"
	}
	return &federation.RefusedAccount{
		Network:    federation.NetworkActivityPub,
		Identifier: actor.ID,
		Handle:     actor.Handle,
		Name:       actor.Name,
	}, 0, ""
}

// Where a Bluesky handle is asked for the DID it stands for: Bluesky's
// public service, which needs no account.
var blueskyResolver = "https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle"

func (h *Handlers) resolveBlueskyHandle(ctx context.Context, handle string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blueskyResolver+"?handle="+url.QueryEscape(handle), nil)
	if err != nil {
		return "", err
	}
	answer, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer answer.Body.Close()
	if answer.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolving %s answered %d", handle, answer.StatusCode)
	}
	var resolved struct {
		DID string `json:"did"`
	}
	if err := json.NewDecoder(answer.Body).Decode(&resolved); err != nil {
		return "", err
	}
	if !strings.HasPrefix(resolved.DID, "did:") {
		return "", nil
	}
	return strings.ToLower(resolved.DID), nil
}

// LetAccountBack stops refusing an account. What was removed does not
// come back; the account can be looked up and followed again.
func (h *Handlers) LetAccountBack(c echo.Context) error {
	if !getViewer(c).IsAdmin() {
		return c.NoContent(http.StatusForbidden)
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		return c.NoContent(http.StatusNotFound)
	}
	account, err := h.RefusedAccounts.Read(id)
	if err != nil {
		log.Println("refused accounts:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	if account == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if err := h.RefusedAccounts.Delete(id); err != nil {
		log.Println("refused accounts:", err)
		return refuse(c, http.StatusInternalServerError, "unavailable")
	}
	return c.NoContent(http.StatusNoContent)
}

// removePhotoPosts removes somebody's photo posts from elsewhere with what
// is stored of their pictures, before they themselves go: the graph's
// removal of them takes the posts, and would leave the pictures.
func (h *Handlers) removePhotoPosts(ownerID int64) {
	if h.PhotoPostsOf == nil {
		return
	}
	ids, err := h.PhotoPostsOf(ownerID)
	if err != nil {
		log.Printf("the photo posts of %d were not read: %v", ownerID, err)
		return
	}
	keeper := h.keeper()
	for _, id := range ids {
		if _, err := keeper.RemovePost(id); err != nil {
			log.Printf("photo post %d of %d was not removed: %v", id, ownerID, err)
		}
	}
}

// announceGone tells the other services that somebody from elsewhere is
// gone, as a purge tells of somebody here: the reports about them are
// closed, and what was chosen about them is forgotten.
func (h *Handlers) announceGone(id int64) {
	if h.Announce == nil || id <= 0 {
		return
	}
	name := fmt.Sprintf(event.EventFormatNodeActorPurged, "activitypub", "person")
	payload := &event.EventPayload{Name: name, ObjectID: id, TargetID: id, SubjectID: id, SourceService: "activitypub-service"}
	h.later(func() {
		if err := h.Announce(payload, name); err != nil {
			log.Printf("saying that %d is gone: %v", id, err)
		}
	})
}
