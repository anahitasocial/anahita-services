package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"activitypub/ap"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
)

const (
	websiteID   = 950
	websiteUUID = "5a0f7c3e-2b1d-4c8e-9f6a-0d3b2e1c4a59"
	articleID   = 960
)

// A website on the fediverse: a site's feed (docs/plans/website.md), public,
// with nobody to choose it, and an article of it made by its administrator,
// Ana, who is not on the fediverse herself.
func websiteWorld() *inboxWorld {
	w := newInboxWorld()

	website := entities.NewAnahitaNode()
	website.ID, website.UUID, website.Alias, website.Name = websiteID, websiteUUID, "garden-notes", "Garden notes"
	website.Type = constants.NodeActorWebsiteEntityType
	website.Access, website.Enabled = "public", true

	article := entities.NewAnahitaNode()
	article.ID, article.UUID = articleID, "1c2b3a49-5d6e-4f70-8a9b-0c1d2e3f4a5b"
	article.Type = constants.NodeMediumArticleEntityType
	article.Name = "Beans & peas"
	article.Body = "Sown in March, up in April.\n\nhttps://garden.example/beans\n\n#Gardens"
	article.Access, article.Enabled = "public", true
	article.OwnerID, article.OwnerType, article.CreatedBy = websiteID, constants.NodeActorWebsiteEntityType, anaID
	article.CreatedAt = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	article.UpdatedAt = article.CreatedAt

	w.nodes.people = append(w.nodes.people, website, article)
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
		t.Errorf("a switched-off Website: %d", rec.Code)
	}
}

func TestAWebsitesArticleGoesOutAsAnArticleOfTheWebsite(t *testing.T) {
	w := websiteWorld()

	err := w.h.HandleEvent(&event.EventPayload{Name: "node.medium.text.article.added.v1", ObjectID: articleID, ObjectType: constants.NodeMediumArticleEntityType})
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
		t.Errorf("sent as %d %v, want the Website", sent.actorID, sent.payload["actor"])
	}
	if object["type"] != "Article" || object["name"] != "Beans & peas" || object["summary"] != "Sown in March, up in April." ||
		object["attributedTo"] != websiteActor || object["url"] != "https://example.org/articles/960" {
		t.Errorf("the article: %v", object)
	}
	if cc, _ := object["cc"].([]any); len(cc) != 1 || cc[0] != websiteActor+"/followers" {
		t.Errorf("cc %v: the Website's followers", object["cc"])
	}
}

func TestOnlyAWebsitesArticlesGoOut(t *testing.T) {
	w := websiteWorld()
	article := w.nodes.people[len(w.nodes.people)-1]
	article.OwnerID, article.OwnerType = anaID, constants.NodeActorPersonEntityType
	w.accounts.enabled[anaID] = true

	_ = w.h.HandleEvent(&event.EventPayload{Name: "node.medium.text.article.added.v1", ObjectID: articleID, ObjectType: constants.NodeMediumArticleEntityType})
	if len(w.queue.sent) != 0 {
		t.Errorf("somebody's own article went out: roadmap row 84")
	}
}
