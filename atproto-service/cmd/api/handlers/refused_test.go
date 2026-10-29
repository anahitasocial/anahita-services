package handlers

import (
	"context"
	"slices"
	"strings"
	"testing"

	"anahita-libs/federation"
)

// The accounts refused, as the shared list keeps them.
type fakeRefusedAccounts struct {
	dids []string
}

func (f *fakeRefusedAccounts) IsRefused(network, identifier string) bool {
	return network == federation.NetworkATProto && slices.Contains(f.dids, federation.CleanIdentifier(network, identifier))
}

func (f *fakeRefusedAccounts) Refused(network string) []string {
	if network != federation.NetworkATProto {
		return nil
	}
	return f.dids
}

func (f *fakeRefusedAccounts) Browse(string, int, int) ([]*federation.RefusedAccount, int, error) {
	return nil, 0, nil
}

func (f *fakeRefusedAccounts) Add(account *federation.RefusedAccount) (*federation.RefusedAccount, error) {
	return account, nil
}

func (f *fakeRefusedAccounts) Read(int64) (*federation.RefusedAccount, error) { return nil, nil }
func (f *fakeRefusedAccounts) Delete(int64) error                            { return nil }

// THE RULE THIS GUARDS: an account on Bluesky refused by its DID is asked
// for no more, is removed with what is here of it, and cannot be looked up
// again, whatever its handle is now.
func TestARefusedBlueskyAccountGoesAndIsHeardNoMore(t *testing.T) {
	w := newWorld(t)
	kept := w.alice(true)
	refused := &fakeRefusedAccounts{}
	w.h.RefusedAccounts = refused

	if wanted, _ := w.h.Wanted(); !slices.Contains(wanted, aliceDID) {
		t.Fatalf("before she is refused, wanted %v", wanted)
	}

	refused.dids = []string{strings.ToLower(aliceDID)}

	if wanted, _ := w.h.Wanted(); slices.Contains(wanted, aliceDID) {
		t.Errorf("a refused account is still asked for: %v", wanted)
	}
	if gone := w.h.ForgetRefused(); gone != 1 {
		t.Fatalf("removed %d", gone)
	}
	if still, _ := w.actors.ReadByDID(aliceDID); still != nil {
		t.Error("the refused account is still kept")
	}
	if w.keptAt("at://"+aliceDID) != nil {
		t.Error("her profile is still in the graph")
	}
	told := strings.Join(w.said, "\n")
	if !strings.Contains(told, "node.actor.atproto.person.purged.v1") {
		t.Errorf("the other services were not told: %s", told)
	}

	// Asked for by her handle, she is not found; nor is anybody removed
	// twice.
	if person, code := w.h.lookup(context.Background(), "alice.bsky.social"); person != nil || code != errLookupBlocked {
		t.Errorf("looked up again: %v %q", person, code)
	}
	if gone := w.h.ForgetRefused(); gone != 0 {
		t.Errorf("removed %d the second time", gone)
	}
	_ = kept
}
