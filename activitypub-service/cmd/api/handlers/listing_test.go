package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"activitypub/data/repositories"
)

func TestBeingListedAndFoundIsOnUntilSomebodySaysOtherwise(t *testing.T) {
	w := newWorld()

	var settings Settings
	_ = json.Unmarshal(w.call(w.h.ReadSettings, http.MethodGet, "/", "", anaID).Body.Bytes(), &settings)
	if !settings.Discoverable || !settings.Indexable {
		t.Fatalf("to begin with %+v", settings)
	}

	var actor map[string]any
	_ = json.Unmarshal(w.get(w.h.Actor, "/", "uuid", anaUUID).Body.Bytes(), &actor)
	if actor["discoverable"] != true || actor["indexable"] != true {
		t.Errorf("other servers are told %v and %v", actor["discoverable"], actor["indexable"])
	}
}

func TestEachCanBeChangedOnItsOwnAndChangedBack(t *testing.T) {
	w := newWorld()

	set := func(body string) (Settings, map[string]any) {
		t.Helper()
		rec := w.call(w.h.EditSettings, http.MethodPut, "/", body, anaID)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
		var settings Settings
		_ = json.Unmarshal(rec.Body.Bytes(), &settings)
		var actor map[string]any
		_ = json.Unmarshal(w.get(w.h.Actor, "/", "uuid", anaUUID).Body.Bytes(), &actor)
		return settings, actor
	}

	settings, actor := set(`{"indexable":false}`)
	if !settings.Discoverable || settings.Indexable {
		t.Errorf("only indexable was changed: %+v", settings)
	}
	if actor["discoverable"] != true || actor["indexable"] != false {
		t.Errorf("other servers are told %v and %v", actor["discoverable"], actor["indexable"])
	}

	settings, actor = set(`{"discoverable":false}`)
	if settings.Discoverable || settings.Indexable {
		t.Errorf("the other stays as it was: %+v", settings)
	}
	if actor["discoverable"] != false {
		t.Errorf("other servers are told %v", actor["discoverable"])
	}

	settings, _ = set(`{"discoverable":true,"indexable":true}`)
	if !settings.Discoverable || !settings.Indexable {
		t.Errorf("changed back %+v", settings)
	}
	if !settings.Enabled {
		t.Error("choosing these took them off the fediverse")
	}
}

type fakeKnownProfiles struct {
	found []*repositories.KnownProfile
}

func (f *fakeKnownProfiles) Browse(string, int, int) ([]*repositories.KnownProfile, int, error) {
	return f.found, len(f.found), nil
}

func TestOnlyAdministratorsSeeWhoIsKeptFromElsewhere(t *testing.T) {
	w := newWorld()
	w.h.KnownProfiles = &fakeKnownProfiles{found: []*repositories.KnownProfile{
		{ID: 9, Network: "activitypub", Name: "Alice", Handle: "alice@gts.test"},
	}}

	if rec := w.callAs("registered", w.h.BrowseKnownProfiles, http.MethodGet, ""); rec.Code != http.StatusForbidden {
		t.Errorf("a registered person: %d", rec.Code)
	}

	rec := w.callAs("administrator", w.h.BrowseKnownProfiles, http.MethodGet, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("an administrator: %d %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Data []repositories.KnownProfile `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &page)
	if len(page.Data) != 1 || page.Data[0].Handle != "alice@gts.test" {
		t.Errorf("page %+v", page)
	}
}
