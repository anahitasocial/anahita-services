package handlers

import (
	"context"
	"errors"
	"testing"

	"actor/data/services"
	"anahita-libs/data/entities"
	"anahita-libs/geocoding"
)

// fakeGeocoder answers with one point, or an error, and counts the asking.
type fakeGeocoder struct {
	asked []string
	err   error
}

func (f *fakeGeocoder) Geocode(_ context.Context, address string) (*geocoding.Location, error) {
	f.asked = append(f.asked, address)
	if f.err != nil {
		return nil, f.err
	}

	return &geocoding.Location{Latitude: 49.28, Longitude: -123.12}, nil
}

func placeHandler(geocoder services.Geocoder) *eventHandler {
	return &eventHandler{groupHandler: &groupHandler{Env: &Env{Srv: &services.Services{Geocoder: geocoder}}}}
}

// An address goes to a mapping service only when a map was asked for.
func TestSetEventPlaceLooksUpOnlyWhenAsked(t *testing.T) {
	geocoder := &fakeGeocoder{}
	actor := &entities.AnahitaNode{}

	placeHandler(geocoder).setEventPlace(actor, &eventFields{Address: " 1 Main St ", City: "Vancouver"})

	if len(geocoder.asked) != 0 {
		t.Fatalf("looked up %v without being asked", geocoder.asked)
	}
	if actor.GeoAddress != "1 Main St" || actor.GeoLatitude != 0 || actor.GeoLongitude != 0 {
		t.Fatalf("got %q at %v,%v", actor.GeoAddress, actor.GeoLatitude, actor.GeoLongitude)
	}

	placeHandler(geocoder).setEventPlace(actor, &eventFields{Address: "1 Main St", City: "Vancouver", ShowMap: true})

	if len(geocoder.asked) != 1 || geocoder.asked[0] != "1 Main St, Vancouver" {
		t.Fatalf("asked %v", geocoder.asked)
	}
	if actor.GeoLatitude != 49.28 || actor.GeoLongitude != -123.12 {
		t.Fatalf("got %v,%v", actor.GeoLatitude, actor.GeoLongitude)
	}
}

// Saving again with the same address keeps the point and asks nobody. A
// changed address is looked up again, and unticking the box takes the point
// away.
func TestSetEventPlaceKeepsChangesAndClears(t *testing.T) {
	geocoder := &fakeGeocoder{}
	handler := placeHandler(geocoder)
	actor := &entities.AnahitaNode{}
	actor.GeoAddress = "1 Main St"
	actor.GeoLatitude, actor.GeoLongitude = 1, 2

	handler.setEventPlace(actor, &eventFields{Address: "1 Main St", ShowMap: true})
	if len(geocoder.asked) != 0 || actor.GeoLatitude != 1 {
		t.Fatalf("asked %v, latitude %v", geocoder.asked, actor.GeoLatitude)
	}

	handler.setEventPlace(actor, &eventFields{Address: "2 Main St", ShowMap: true})
	if len(geocoder.asked) != 1 || actor.GeoLatitude != 49.28 {
		t.Fatalf("asked %v, latitude %v", geocoder.asked, actor.GeoLatitude)
	}

	handler.setEventPlace(actor, &eventFields{Address: "2 Main St"})
	if actor.GeoLatitude != 0 || actor.GeoLongitude != 0 || actor.GeoAddress != "2 Main St" {
		t.Fatalf("got %q at %v,%v", actor.GeoAddress, actor.GeoLatitude, actor.GeoLongitude)
	}

	actor.GeoLatitude, actor.GeoLongitude = 1, 2
	handler.setEventPlace(actor, &eventFields{ShowMap: true})
	if actor.GeoLatitude != 0 || actor.GeoLongitude != 0 {
		t.Fatalf("a point left with no address: %v,%v", actor.GeoLatitude, actor.GeoLongitude)
	}
}

// An address nobody can find, or no geocoder at all, is kept without a point.
func TestSetEventPlaceWithoutAnAnswer(t *testing.T) {
	for name, handler := range map[string]*eventHandler{
		"not found":   placeHandler(&fakeGeocoder{err: errors.New("no")}),
		"no geocoder": placeHandler(nil),
	} {
		actor := &entities.AnahitaNode{}
		actor.GeoLatitude, actor.GeoLongitude = 1, 2
		actor.GeoAddress = "old"

		handler.setEventPlace(actor, &eventFields{Address: "Nowhere", ShowMap: true})

		if actor.GeoAddress != "Nowhere" || actor.GeoLatitude != 0 || actor.GeoLongitude != 0 {
			t.Fatalf("%s: got %q at %v,%v", name, actor.GeoAddress, actor.GeoLatitude, actor.GeoLongitude)
		}
	}
}
