package handlers

import (
	"anahita-libs/federation"
	"atproto/bsky"
	"atproto/data/repositories"
	"context"
	"errors"
	"log"
	"sync"
)

// The short tokens, one for each connected person, kept only in memory.
type tokens struct {
	mu   sync.Mutex
	held map[int64]string
}

func (h *Handlers) remember(personID int64, token string) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	if h.tokens.held == nil {
		h.tokens.held = map[int64]string{}
	}
	h.tokens.held[personID] = token
}

func (h *Handlers) forgetToken(personID int64) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	delete(h.tokens.held, personID)
}

// token is the short token to write as an account with: the one held, or
// a new one got with the long token when there is none or `fresh` is
// asked for. An account whose long token is refused is marked as needing
// to be connected again.
func (h *Handlers) token(ctx context.Context, account *repositories.Account, fresh bool) (string, error) {
	h.tokens.mu.Lock()
	held := h.tokens.held[account.PersonID]
	h.tokens.mu.Unlock()
	if held != "" && !fresh {
		return held, nil
	}

	// The long token as it is kept now: another copy of this service may
	// have spent the one this copy read.
	current, err := h.Accounts.Read(account.PersonID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", federation.Permanent(errors.New("no Bluesky account is connected"))
	}
	refresh, err := h.Crypt.Decrypt(current.Refresh)
	if err != nil {
		return "", federation.Permanent(err)
	}

	session, err := h.Writer.RefreshSession(ctx, current.PDS, refresh)
	if errors.Is(err, bsky.ErrDenied) || errors.Is(err, bsky.ErrExpired) {
		log.Printf("the Bluesky account of %d has to be connected again: %v", account.PersonID, err)
		if err := h.Accounts.SetBroken(account.PersonID); err != nil {
			log.Println("marking it so:", err)
		}
		h.forgetToken(account.PersonID)
		return "", federation.Permanent(err)
	}
	if err != nil {
		return "", err
	}

	kept, err := h.Crypt.Encrypt(session.RefreshJWT)
	if err != nil {
		return "", err
	}
	if err := h.Accounts.SetRefresh(account.PersonID, kept); err != nil {
		return "", err
	}
	h.remember(account.PersonID, session.AccessJWT)
	return session.AccessJWT, nil
}
