package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"

	"github.com/labstack/echo/v4"
)

const gardenID = 900

// adminRelations says who administers what: Read(actor, viewer).HasAdmin
// when the viewer is one of the actor's administrators.
type adminRelations struct {
	fakeRelations
	admins map[int64]int64
}

func (f adminRelations) Read(a, b int64) (*entities.ActorRelation, error) {
	return &entities.ActorRelation{HasAdmin: f.admins[a] == b}, nil
}

func whoseAs(h *Handlers, viewerID int64, profile string) (int64, int) {
	req := httptest.NewRequest(http.MethodGet, "/atproto/settings?profile_id="+profile, nil)
	c := echo.New().NewContext(req, httptest.NewRecorder())
	viewer := entities.NewViewer()
	viewer.ID, viewer.Authenticated = viewerID, true
	c.Set(constants.ViewerContextKey, viewer)
	return h.whose(c)
}

// A website's own Bluesky account is connected by one of its administrators,
// and its notes then go there as that account; what the account posts on
// Bluesky itself is not made a post of the website here.
func TestAWebsitesBlueskyAccount(t *testing.T) {
	w, _ := writingWorld(t)
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: gardenID, Type: constants.NodeActorWebsiteEntityType, Alias: "garden", Enabled: true, Access: "public",
	})
	w.h.Relations = adminRelations{admins: map[int64]int64{gardenID: anaID}}

	if id, status := whoseAs(w.h, anaID, ""); id != anaID || status != 0 {
		t.Errorf("her own: %d %d", id, status)
	}
	if id, status := whoseAs(w.h, anaID, "900"); id != gardenID || status != 0 {
		t.Errorf("the website she looks after: %d %d", id, status)
	}
	if _, status := whoseAs(w.h, 8, "900"); status != http.StatusForbidden {
		t.Errorf("somebody who does not look after it: %d", status)
	}
	if _, status := whoseAs(w.h, 8, "7"); status != http.StatusNotFound {
		t.Errorf("a person who is not a website: %d", status)
	}

	w.accounts.kept[gardenID] = &repositories.Account{PersonID: gardenID, DID: "did:plc:garden", Handle: "garden.example.org", PDS: anaPDS, Refresh: "enc(refresh)"}

	note := &entities.AnahitaNode{
		ID: 70, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: gardenID, CreatedBy: gardenID,
		OwnerType: constants.NodeActorWebsiteEntityType, Access: "public", Body: "**Beans**\n\nhttps://garden.example/beans",
	}
	w.nodes.kept = append(w.nodes.kept, note)
	w.heard("node.medium.text-service.note.added.v1", event.EventPayload{ObjectID: 70, ObjectType: constants.NodeMediumNoteEntityType})
	if len(w.queue.waiting) != 1 {
		t.Errorf("queued %d: a website's note goes to its Bluesky account", len(w.queue.waiting))
	}

	w.h.Media = &fakeMedia{nodes: w.nodes}
	before := len(w.nodes.kept)
	if err := w.h.ownPosted(w.accounts.kept[gardenID], "3l3own", aPost(t, `{"text":"Posted on Bluesky itself"}`)); err != nil {
		t.Fatal(err)
	}
	if len(w.nodes.kept) != before {
		t.Errorf("what the website's account posted on Bluesky became a post of it")
	}

	// Somebody's own, the same way, is theirs here: the rule is the
	// website's alone.
	w.connect()
	if err := w.h.ownPosted(w.accounts.kept[anaID], "3l3ana", aPost(t, `{"text":"Ana on Bluesky"}`)); err != nil {
		t.Fatal(err)
	}
	if len(w.nodes.kept) != before+1 {
		t.Errorf("a person's own post on Bluesky was not kept as theirs: %d nodes, want %d", len(w.nodes.kept), before+1)
	}
	_ = context.Background()
}
