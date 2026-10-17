package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"activitypub/ap"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"

	"github.com/labstack/echo/v4"
)

const (
	websiteID   = 950
	websiteUUID = "5a0f7c3e-2b1d-4c8e-9f6a-0d3b2e1c4a59"
	itemID      = 960
)

// A website on the fediverse: a site's feed (docs/websites.md), public,
// with nobody to choose it, and a note of it, the website's own, made by
// its administrator Ana, who is not on the fediverse herself.
func websiteWorld() *inboxWorld {
	w := newInboxWorld()

	website := entities.NewAnahitaNode()
	website.ID, website.UUID, website.Alias, website.Name = websiteID, websiteUUID, "garden-notes", "Garden notes"
	website.Type = constants.NodeActorWebsiteEntityType
	website.Access, website.Enabled = "public", true

	item := entities.NewAnahitaNode()
	item.ID, item.UUID = itemID, "1c2b3a49-5d6e-4f70-8a9b-0c1d2e3f4a5b"
	item.Type = constants.NodeMediumNoteEntityType
	item.Body = "**Beans & peas**\n\nSown in March, up in April.\n\nhttps://garden.example/beans\n\n#Gardens"
	item.Access, item.Enabled = "public", true
	item.OwnerID, item.OwnerType, item.CreatedBy = websiteID, constants.NodeActorWebsiteEntityType, websiteID
	item.CreatedAt = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	item.UpdatedAt = item.CreatedAt

	w.nodes.people = append(w.nodes.people, website, item)
	w.accounts.enabled[anaID] = false
	w.follows.inboxes = []string{"https://mastodon.example/inbox"}
	return w
}

func TestAWebsiteIsAnOrganizationFoundByItsHandle(t *testing.T) {
	w := websiteWorld()

	rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource=acct:garden-notes@example.org")
	var jrd ap.JRD
	_ = json.Unmarshal(rec.Body.Bytes(), &jrd)
	if rec.Code != http.StatusOK || len(jrd.Links) == 0 || jrd.Links[0].Href != "https://api.example.org/ap/users/"+websiteUUID {
		t.Fatalf("WebFinger: %d %+v", rec.Code, jrd)
	}

	rec = w.get(w.h.Actor, "/ap/users/"+websiteUUID, "uuid", websiteUUID)
	var actor map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &actor)
	if rec.Code != http.StatusOK || actor["type"] != "Organization" || actor["preferredUsername"] != "garden-notes" {
		t.Errorf("the actor: %d %v", rec.Code, actor)
	}

	// A website switched off is not there.
	w.nodes.people[len(w.nodes.people)-2].Enabled = false
	if rec := w.get(w.h.Actor, "/ap/users/"+websiteUUID, "uuid", websiteUUID); rec.Code != http.StatusNotFound {
		t.Errorf("a switched-off website: %d", rec.Code)
	}
}

func TestAWebsitesNoteGoesOutAsTheWebsites(t *testing.T) {
	w := websiteWorld()

	err := w.h.HandleEvent(&event.EventPayload{Name: "node.medium.text.note.added.v1", ObjectID: itemID, ObjectType: constants.NodeMediumNoteEntityType})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) != 1 {
		t.Fatalf("queued %d", len(w.queue.sent))
	}
	sent := w.queue.sent[0]
	websiteActor := "https://api.example.org/ap/users/" + websiteUUID
	object, _ := sent.payload["object"].(map[string]any)
	if sent.actorID != websiteID || sent.payload["actor"] != websiteActor {
		t.Errorf("sent as %d %v, want the website", sent.actorID, sent.payload["actor"])
	}
	content, _ := object["content"].(string)
	if object["type"] != "Note" || object["attributedTo"] != websiteActor || !strings.Contains(content, "<strong>Beans &amp; peas</strong>") {
		t.Errorf("the note: %v", object)
	}
	if cc, _ := object["cc"].([]any); len(cc) != 1 || cc[0] != websiteActor+"/followers" {
		t.Errorf("cc %v: the website's followers", object["cc"])
	}
}

// A website's administrators see its handle under its settings, and that it
// is on the fediverse, which nobody chose; anybody else is refused.
func TestAWebsitesAdministratorsSeeItsHandle(t *testing.T) {
	w := websiteWorld()
	ask := func(viewer int64) *httptest.ResponseRecorder {
		ec := echo.New()
		rec := httptest.NewRecorder()
		c := ec.NewContext(httptest.NewRequest(http.MethodGet, "/activitypub/settings?person_id=950", nil), rec)
		v := entities.NewViewer()
		v.ID, v.PersonType = viewer, "registered"
		c.Set(constants.ViewerContextKey, v)
		_ = w.h.ReadSettings(c)
		return rec
	}

	w.relations.relation = &entities.ActorRelation{ActorAId: websiteID, ActorBId: anaID, HasAdmin: true}
	rec := ask(anaID)
	var settings Settings
	_ = json.Unmarshal(rec.Body.Bytes(), &settings)
	if rec.Code != http.StatusOK || !settings.Enabled || !settings.Available || settings.Handle != "garden-notes@example.org" {
		t.Errorf("its administrator: %d %+v", rec.Code, settings)
	}

	// Switched off, it is not.
	for _, node := range w.nodes.people {
		if node.ID == websiteID {
			node.Enabled = false
		}
	}
	settings = Settings{}
	_ = json.Unmarshal(ask(anaID).Body.Bytes(), &settings)
	if settings.Enabled || settings.Available || settings.Reason != reasonProfileNotPublic {
		t.Errorf("switched off: %+v", settings)
	}

	w.relations.relation = &entities.ActorRelation{ActorAId: websiteID, ActorBId: 99}
	if rec := ask(99); rec.Code != http.StatusForbidden {
		t.Errorf("somebody else: %d", rec.Code)
	}
}
