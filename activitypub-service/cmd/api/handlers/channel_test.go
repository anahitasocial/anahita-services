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
	channelID   = 950
	channelUUID = "5a0f7c3e-2b1d-4c8e-9f6a-0d3b2e1c4a59"
	itemID      = 960
)

// A channel on the fediverse: a site's feed (docs/channels.md), public, put
// on the fediverse by its administrators, and a note of it, the channel's
// own, made by its administrator Ana, who is not on the fediverse herself.
func channelWorld() *inboxWorld {
	w := newInboxWorld()

	channel := entities.NewAnahitaNode()
	channel.ID, channel.UUID, channel.Alias, channel.Name = channelID, channelUUID, "garden-notes", "Garden notes"
	channel.Type = constants.NodeActorChannelEntityType
	channel.Access, channel.Enabled = "public", true

	item := entities.NewAnahitaNode()
	item.ID, item.UUID = itemID, "1c2b3a49-5d6e-4f70-8a9b-0c1d2e3f4a5b"
	item.Type = constants.NodeMediumNoteEntityType
	item.Body = "**Beans & peas**\n\nSown in March, up in April.\n\nhttps://garden.example/beans\n\n#Gardens"
	item.Access, item.Enabled = "public", true
	item.OwnerID, item.OwnerType, item.CreatedBy = channelID, constants.NodeActorChannelEntityType, channelID
	item.CreatedAt = time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	item.UpdatedAt = item.CreatedAt

	w.nodes.people = append(w.nodes.people, channel, item)
	w.accounts.enabled[anaID] = false
	w.accounts.enabled[channelID] = true
	w.follows.inboxes = []string{"https://mastodon.example/inbox"}
	return w
}

func TestAChannelIsAnOrganizationFoundByItsHandle(t *testing.T) {
	w := channelWorld()

	rec := w.get(w.h.WebFinger, "/.well-known/webfinger?resource=acct:garden-notes@example.org")
	var jrd ap.JRD
	_ = json.Unmarshal(rec.Body.Bytes(), &jrd)
	if rec.Code != http.StatusOK || len(jrd.Links) == 0 || jrd.Links[0].Href != "https://api.example.org/ap/users/"+channelUUID {
		t.Fatalf("WebFinger: %d %+v", rec.Code, jrd)
	}

	rec = w.get(w.h.Actor, "/ap/users/"+channelUUID, "uuid", channelUUID)
	var actor map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &actor)
	if rec.Code != http.StatusOK || actor["type"] != "Organization" || actor["preferredUsername"] != "garden-notes" {
		t.Errorf("the actor: %d %v", rec.Code, actor)
	}

	// A channel switched off is not there.
	w.nodes.people[len(w.nodes.people)-2].Enabled = false
	if rec := w.get(w.h.Actor, "/ap/users/"+channelUUID, "uuid", channelUUID); rec.Code != http.StatusNotFound {
		t.Errorf("a switched-off channel: %d", rec.Code)
	}
}

func TestAChannelsNoteGoesOutAsTheChannels(t *testing.T) {
	w := channelWorld()

	err := w.h.HandleEvent(&event.EventPayload{Name: "node.medium.text.note.added.v1", ObjectID: itemID, ObjectType: constants.NodeMediumNoteEntityType})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.queue.sent) != 1 {
		t.Fatalf("queued %d", len(w.queue.sent))
	}
	sent := w.queue.sent[0]
	channelActor := "https://api.example.org/ap/users/" + channelUUID
	object, _ := sent.payload["object"].(map[string]any)
	if sent.actorID != channelID || sent.payload["actor"] != channelActor {
		t.Errorf("sent as %d %v, want the channel", sent.actorID, sent.payload["actor"])
	}
	content, _ := object["content"].(string)
	if object["type"] != "Note" || object["attributedTo"] != channelActor || !strings.Contains(content, "<strong>Beans &amp; peas</strong>") {
		t.Errorf("the note: %v", object)
	}
	if cc, _ := object["cc"].([]any); len(cc) != 1 || cc[0] != channelActor+"/followers" {
		t.Errorf("cc %v: the channel's followers", object["cc"])
	}
}

// A channel is off the fediverse until one of its administrators puts it
// on, as a person is until they choose to be; they see its handle, and
// take it off again. Anybody else, a site administrator among them, may not
// make that choice for it.
func TestAChannelsAdministratorsPutItOnTheFediverse(t *testing.T) {
	w := channelWorld()
	w.accounts.enabled[channelID] = false
	call := func(handler echo.HandlerFunc, method string, viewer int64, personType, body string) (*httptest.ResponseRecorder, Settings) {
		ec := echo.New()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/activitypub/settings?person_id=950", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		c := ec.NewContext(req, rec)
		v := entities.NewViewer()
		v.ID, v.PersonType = viewer, personType
		c.Set(constants.ViewerContextKey, v)
		_ = handler(c)
		var settings Settings
		_ = json.Unmarshal(rec.Body.Bytes(), &settings)
		return rec, settings
	}
	w.relations.relation = &entities.ActorRelation{ActorAId: channelID, ActorBId: anaID, HasAdmin: true}

	rec, settings := call(w.h.ReadSettings, http.MethodGet, anaID, "registered", "")
	if rec.Code != http.StatusOK || settings.Enabled || !settings.Available || settings.Handle != "garden-notes@example.org" {
		t.Errorf("off until chosen: %d %+v", rec.Code, settings)
	}
	if w.h.federating(w.nodes.people[len(w.nodes.people)-2]) != nil {
		t.Errorf("found on the fediverse before it is put there")
	}

	rec, settings = call(w.h.EditSettings, http.MethodPut, anaID, "registered", `{"enabled":true}`)
	if rec.Code != http.StatusOK || !settings.Enabled || !w.accounts.enabled[channelID] || w.accounts.enabled[anaID] {
		t.Errorf("put on by its administrator: %d %+v, channel %v, Ana %v", rec.Code, settings, w.accounts.enabled[channelID], w.accounts.enabled[anaID])
	}

	rec, settings = call(w.h.EditSettings, http.MethodPut, anaID, "registered", `{"enabled":false}`)
	if rec.Code != http.StatusOK || settings.Enabled || w.accounts.enabled[channelID] {
		t.Errorf("taken off: %d %+v", rec.Code, settings)
	}

	w.relations.relation = &entities.ActorRelation{ActorAId: channelID, ActorBId: 99}
	if rec, _ := call(w.h.ReadSettings, http.MethodGet, 99, "registered", ""); rec.Code != http.StatusForbidden {
		t.Errorf("somebody else reading: %d", rec.Code)
	}
	if rec, _ := call(w.h.EditSettings, http.MethodPut, 99, "administrator", `{"enabled":true}`); rec.Code != http.StatusForbidden || w.accounts.enabled[channelID] {
		t.Errorf("a site administrator choosing for it: %d", rec.Code)
	}
}
