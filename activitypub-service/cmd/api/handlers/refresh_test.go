package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"
)

// THE RULE THIS GUARDS (roadmap row 130): opening the profile of somebody
// elsewhere reads them again when what was kept is more than a day old, so
// that a change there (approving followers, or no longer) is seen here
// without anybody knowing to look them up; and not more often than that.
func TestAnOldProfileFromElsewhereIsReadAgain(t *testing.T) {
	w := newInboxWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	if alice == nil {
		t.Fatal("alice was not kept")
	}
	id := fmt.Sprint(alice.NodeID)

	// Kept a moment ago, as not approving her followers: left as it is.
	alice.Locked, alice.FetchedAt = false, time.Now()
	rec := w.call(w.h.RefreshPerson, http.MethodPost, "/activitypub/people/"+id+"/refresh", "", anaID, "id", id)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"refreshed":false`) {
		t.Fatalf("fresh: %d %s", rec.Code, rec.Body)
	}
	if w.actors.kept[alice.NodeID].Locked {
		t.Error("a profile kept a moment ago was read again")
	}

	// Kept two days ago: read again, and her server says she approves them.
	w.actors.kept[alice.NodeID].FetchedAt = time.Now().Add(-48 * time.Hour)
	rec = w.call(w.h.RefreshPerson, http.MethodPost, "/activitypub/people/"+id+"/refresh", "", anaID, "id", id)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"refreshed":true`) {
		t.Fatalf("old: %d %s", rec.Code, rec.Body)
	}
	if !w.actors.kept[alice.NodeID].Locked {
		t.Error("what her server says now was not kept")
	}

	// Nobody from elsewhere under that id.
	if rec = w.call(w.h.RefreshPerson, http.MethodPost, "/activitypub/people/9999/refresh", "", anaID, "id", "9999"); rec.Code != http.StatusNotFound {
		t.Errorf("nobody: %d", rec.Code)
	}
}
