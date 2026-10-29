package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"anahita-libs/event"
	"anahita-libs/federation"
)

// The accounts refused, as the shared list keeps them.
type fakeRefused struct {
	next     int64
	accounts []*federation.RefusedAccount
}

func (f *fakeRefused) IsRefused(network, identifier string) bool {
	identifier = federation.CleanIdentifier(network, identifier)
	for _, each := range f.accounts {
		if each.Network == network && each.Identifier == identifier {
			return true
		}
	}
	return false
}

func (f *fakeRefused) Refused(network string) []string {
	list := []string{}
	for _, each := range f.accounts {
		if each.Network == network {
			list = append(list, each.Identifier)
		}
	}
	return list
}

func (f *fakeRefused) Browse(search string, limit, offset int) ([]*federation.RefusedAccount, int, error) {
	return f.accounts, len(f.accounts), nil
}

func (f *fakeRefused) Add(account *federation.RefusedAccount) (*federation.RefusedAccount, error) {
	account.Identifier = federation.CleanIdentifier(account.Network, account.Identifier)
	if f.IsRefused(account.Network, account.Identifier) {
		return nil, federation.ErrAlreadyRefused
	}
	f.next++
	account.ID, account.CreatedAt = f.next, time.Now()
	f.accounts = append(f.accounts, account)
	return account, nil
}

func (f *fakeRefused) Read(id int64) (*federation.RefusedAccount, error) {
	for _, each := range f.accounts {
		if each.ID == id {
			return each, nil
		}
	}
	return nil, nil
}

func (f *fakeRefused) Delete(id int64) error {
	left := f.accounts[:0]
	for _, each := range f.accounts {
		if each.ID != id {
			left = append(left, each)
		}
	}
	f.accounts = left
	return nil
}

// THE RULE THIS GUARDS: an account refused by itself is not listened to,
// though nothing about its server is refused.
func TestARefusedAccountIsNotListenedTo(t *testing.T) {
	w := newInboxWorld()
	refused := &fakeRefused{}
	w.h.RefusedAccounts = refused

	_, _ = refused.Add(&federation.RefusedAccount{Network: federation.NetworkActivityPub, Identifier: aliceID})
	if rec := w.fromAlice(follow()); rec.Code != http.StatusForbidden {
		t.Errorf("a follow from a refused account answered %d", rec.Code)
	}
	if len(w.follows.kept) != 0 {
		t.Errorf("a refused account's follow was kept: %v", w.follows.kept)
	}

	// Nor looked up, nor kept, however it is asked for.
	if _, why := w.h.find(context.Background(), "@alice@mastodon.example"); why != errLookupBlocked {
		t.Errorf("a refused account was found: %q", why)
	}

	// On Bluesky, the same address is nobody refused.
	if refused.IsRefused(federation.NetworkATProto, aliceID) {
		t.Error("an address refused on the fediverse is refused on Bluesky")
	}
}

// THE RULE THIS GUARDS: an administrator refuses an account, and what is
// here of it goes with it; the other services are told, as of a purge.
func TestAnAdministratorRefusesAnAccountAndItGoes(t *testing.T) {
	w, alice := settledWorld()
	refused := &fakeRefused{}
	w.h.RefusedAccounts = refused
	var told []string
	w.h.Announce = func(payload *event.EventPayload, name string) error {
		told = append(told, name)
		return nil
	}

	body := `{"actor_id":` + strconv.FormatInt(alice.NodeID, 10) + `,"reason":"Spam"}`

	// Not somebody who is not an administrator.
	if rec := w.call(w.h.RefuseAccount, http.MethodPost, "/", body, anaID); rec.Code != http.StatusForbidden || len(refused.accounts) != 0 {
		t.Fatalf("by somebody who is not an administrator: %d", rec.Code)
	}

	rec := w.callAs("administrator", w.h.RefuseAccount, http.MethodPost, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var added federation.RefusedAccount
	_ = json.Unmarshal(rec.Body.Bytes(), &added)
	if added.Network != federation.NetworkActivityPub || added.Identifier != aliceID || added.Reason != "Spam" || added.Handle == "" {
		t.Errorf("refused as %+v", added)
	}

	// She is gone, with her follows both ways, and the others were told.
	if kept, _ := w.actors.ReadByURI(aliceID); kept != nil {
		t.Error("the refused account is still kept")
	}
	if len(w.follows.kept) != 0 {
		t.Errorf("follows left: %v", w.follows.kept)
	}
	if len(told) != 1 || !strings.HasPrefix(told[0], "node.actor.activitypub.person.purged") {
		t.Errorf("told %v", told)
	}

	// Twice is once; a profile here is not refused.
	if rec := w.callAs("administrator", w.h.RefuseAccount, http.MethodPost, `{"address":"`+aliceID+`"}`); rec.Code != http.StatusConflict {
		t.Errorf("refused twice: %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.callAs("administrator", w.h.RefuseAccount, http.MethodPost, `{"actor_id":`+strconv.FormatInt(anaID, 10)+`}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "is_local") {
		t.Errorf("a profile here: %d %s", rec.Code, rec.Body.String())
	}

	// Listed, and let back; she can then be found again.
	if rec := w.callAs("administrator", w.h.BrowseRefusedAccounts, http.MethodGet, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Errorf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.callAs("administrator", w.h.LetAccountBack, http.MethodDelete, "", "id", strconv.FormatInt(added.ID, 10)); rec.Code != http.StatusNoContent || len(refused.accounts) != 0 {
		t.Errorf("let back: %d", rec.Code)
	}
	if _, why := w.h.find(context.Background(), "@alice@mastodon.example"); why != "" {
		t.Errorf("let back, she is not found: %q", why)
	}
}

// A Bluesky account is refused by its DID, however it is written, and is
// left for atproto-service to remove.
func TestABlueskyAccountIsRefusedByItsDID(t *testing.T) {
	w := newInboxWorld()
	refused := &fakeRefused{}
	w.h.RefusedAccounts = refused

	rec := w.callAs("administrator", w.h.RefuseAccount, http.MethodPost, `{"address":"DID:plc:Eve123","reason":"Harassment"}`)
	if rec.Code != http.StatusCreated || !refused.IsRefused(federation.NetworkATProto, "did:plc:eve123") {
		t.Errorf("%d %s, list %v", rec.Code, rec.Body.String(), refused.accounts)
	}
}
