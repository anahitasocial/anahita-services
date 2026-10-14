package handlers

import (
	"anahita-libs/policy"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/ratelimit"

	"github.com/labstack/echo/v4"
)

// A person's own Bluesky account, connected.
//
// Somebody here who has an account on Bluesky can connect it. From then
// on, what they do here to something that came from Bluesky is done there
// too, as that account: a like, a repost, a reply. Whoever has not
// connected one reads Bluesky and writes nothing to it.
//
// They give their handle and a password made for an app, which Bluesky
// lets an account make and take back without touching its own password.
// It is exchanged for a token at the account's server and is not kept;
// the token is kept encrypted. Disconnecting forgets it.

// How often somebody may try to connect. Each try sends a password to
// another server.
var connectLimit = ratelimit.Limit{Max: 10, Window: 10 * time.Minute}

// Settings is somebody's connection to Bluesky, as the web app is told.
type Settings struct {
	// Whether an account is connected.
	Connected bool   `json:"connected"`
	Handle    string `json:"handle"`
	DID       string `json:"did"`
	// The account's server no longer takes what was kept: it has to be
	// connected again.
	Broken bool `json:"broken"`
}

// Why an account could not be connected.
const (
	errConnectUnreadable = "bluesky_unreadable"
	errConnectNobody     = "bluesky_not_found"
	errConnectDenied     = "bluesky_denied"
	errConnectTaken      = "bluesky_taken"
	errConnectFailed     = "bluesky_failed"
	errConnectLimit      = "too_many_tries"
	errConnectOff        = "bluesky_writing_off"
)

func settingsOf(account *repositories.Account) *Settings {
	if account == nil {
		return &Settings{}
	}
	return &Settings{Connected: true, Handle: account.Handle, DID: account.DID, Broken: account.BrokenAt != nil}
}

// canWrite says whether this installation writes to Bluesky at all.
//
// Only a site open to read writes anything to Bluesky: on one kept to its
// members, or one that shows visitors a preview, nothing may leave, as for
// the fediverse (docs/privacy.md). Connecting is not offered there either.
func (h *Handlers) canWrite() bool {
	if policy.SiteReadAccess() != policy.ReadPublic {
		return false
	}
	return h.Accounts != nil && h.Sent != nil && h.Writer != nil && h.Crypt != nil && h.Queue != nil
}

// ReadSettings says whether the viewer has connected an account.
func (h *Handlers) ReadSettings(c echo.Context) error {
	if !h.canWrite() {
		return refuse(c, http.StatusNotFound, errConnectOff)
	}
	account, err := h.Accounts.Read(getViewer(c).ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, settingsOf(account))
}

type connectRequest struct {
	Handle      string `json:"handle"`
	AppPassword string `json:"app_password"`
}

// EditSettings connects the viewer's Bluesky account.
func (h *Handlers) EditSettings(c echo.Context) error {
	viewer := getViewer(c)
	if !h.canWrite() {
		return refuse(c, http.StatusNotFound, errConnectOff)
	}

	req := &connectRequest{}
	if err := c.Bind(req); err != nil {
		return refuse(c, http.StatusBadRequest, errConnectUnreadable)
	}
	actor := bsky.ParseActor(req.Handle)
	password := strings.TrimSpace(req.AppPassword)
	if actor == "" || password == "" || len(password) > 200 {
		return refuse(c, http.StatusBadRequest, errConnectUnreadable)
	}
	if h.refused(actor) {
		return refuse(c, http.StatusNotFound, errLookupBlocked)
	}
	if h.Limiter != nil && ratelimit.Exceeded(h.Limiter, "atproto_connect", fmt.Sprint(viewer.ID), connectLimit) {
		return refuse(c, http.StatusTooManyRequests, errConnectLimit)
	}

	// Only somebody of this installation connects an account.
	person, err := h.Nodes.Read(viewer.ID)
	if err != nil || person == nil || person.ID == 0 || person.IsRemote() {
		return refuse(c, http.StatusForbidden, errConnectUnreadable)
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*bsky.Timeout)
	defer cancel()

	account, code := h.connect(ctx, viewer.ID, actor, password)
	if account == nil {
		status := http.StatusBadGateway
		switch code {
		case errConnectNobody:
			status = http.StatusNotFound
		case errConnectDenied:
			status = http.StatusForbidden
		case errConnectTaken:
			status = http.StatusConflict
		}
		return refuse(c, status, code)
	}
	return c.JSON(http.StatusOK, settingsOf(account))
}

// connect proves an account is whoever asks' own, by its password, and
// keeps what is needed to write as it.
func (h *Handlers) connect(ctx context.Context, personID int64, actor, password string) (*repositories.Account, string) {
	profile, err := h.Bluesky.Profile(ctx, actor)
	switch {
	case errors.Is(err, bsky.ErrNobody):
		return nil, errConnectNobody
	case err != nil:
		log.Println("connecting a Bluesky account:", err)
		return nil, errConnectFailed
	}

	pds, err := h.Writer.PDSOf(ctx, h.Directory, profile.DID)
	if err != nil {
		log.Printf("connecting %s: its server: %v", profile.DID, err)
		return nil, errConnectFailed
	}
	session, err := h.Writer.CreateSession(ctx, pds, profile.DID, password)
	switch {
	case errors.Is(err, bsky.ErrDenied):
		return nil, errConnectDenied
	case err != nil:
		log.Printf("connecting %s at %s: %v", profile.DID, pds, err)
		return nil, errConnectFailed
	}
	// The server answered for the account that was asked about, and no
	// other.
	if session.DID != profile.DID || session.RefreshJWT == "" {
		return nil, errConnectDenied
	}

	refresh, err := h.Crypt.Encrypt(session.RefreshJWT)
	if err != nil {
		log.Println("connecting a Bluesky account: keeping its token:", err)
		return nil, errConnectFailed
	}
	account := &repositories.Account{
		PersonID: personID, DID: profile.DID, Handle: strings.ToLower(profile.Handle), PDS: pds, Refresh: refresh,
	}
	err = h.Accounts.Save(account)
	switch {
	case errors.Is(err, repositories.ErrAccountTaken):
		return nil, errConnectTaken
	case err != nil:
		log.Println("connecting a Bluesky account:", err)
		return nil, errConnectFailed
	}
	h.remember(personID, session.AccessJWT)
	// They are one person here from now on.
	h.later(func() { h.forgetShadow(profile.DID) })
	// Their own posts on Bluesky are listened for from now on.
	h.changed()
	return account, ""
}

// DeleteSettings disconnects the viewer's account. What was written to
// Bluesky as it stays there, and can no longer be taken back from here.
func (h *Handlers) DeleteSettings(c echo.Context) error {
	viewer := getViewer(c)
	if !h.canWrite() {
		return refuse(c, http.StatusNotFound, errConnectOff)
	}
	if err := h.Accounts.Delete(viewer.ID); err != nil {
		return err
	}
	if err := h.Sent.DeleteAllOf(viewer.ID); err != nil {
		return err
	}
	h.forgetToken(viewer.ID)
	return c.JSON(http.StatusOK, &Settings{})
}
