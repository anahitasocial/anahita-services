package federation

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAPlaceIsReadAsTheVocabularyHasIt(t *testing.T) {
	// As Pixelfed sends it: `country` is its own, and not read.
	got := ReadPlace(json.RawMessage(`{"type":"Place","name":"Vancouver","latitude":49.28,"longitude":-123.12,"country":"Canada"}`))
	if got == nil || got.Name != "Vancouver" || got.Latitude != 49.28 || got.Longitude != -123.12 {
		t.Fatalf("got %+v", got)
	}

	// The first that is a place with a point, of several.
	many := ReadPlace(json.RawMessage(`[{"type":"Link","href":"https://example.org"},{"type":["Place"],"latitude":1,"longitude":2}]`))
	if many == nil || many.Latitude != 1 {
		t.Errorf("of several: %+v", many)
	}

	for _, none := range []string{
		``, `null`,
		`{"type":"Place","name":"Somewhere"}`,
		`{"type":"Place","latitude":91,"longitude":0}`,
		`{"type":"Note","latitude":1,"longitude":2}`,
		`"Vancouver"`,
	} {
		if p := ReadPlace(json.RawMessage(none)); p != nil {
			t.Errorf("%q read as %+v", none, p)
		}
	}
}

func TestAPlaceIsSentAsTheVocabularyHasIt(t *testing.T) {
	raw, _ := json.Marshal(Place{Name: "Here", Latitude: 1.5, Longitude: -2}.AsLocation())
	back := ReadPlace(raw)
	if back == nil || back.Name != "Here" || back.Latitude != 1.5 || back.Longitude != -2 {
		t.Errorf("sent %s, read back %+v", raw, back)
	}
	unnamed, _ := json.Marshal(Place{Latitude: 1, Longitude: 2}.AsLocation())
	if string(unnamed) != `{"latitude":1,"longitude":2,"type":"Place"}` {
		t.Errorf("unnamed: %s", unnamed)
	}
}

// Every place a post from elsewhere names (roadmap row 82, part 4): its
// location first, then the Places in its tag, each once, at most five, and
// nothing that is not a Place with a point.
func TestReadPlaces(t *testing.T) {
	location := json.RawMessage(`{"type":"Place","name":"Paris","latitude":48.8566,"longitude":2.3522,"country":"France"}`)
	tag := json.RawMessage(`[
		{"type":"Hashtag","name":"#paris","href":"https://pixelfed.social/discover/tags/paris"},
		{"type":"Place","id":"https://places.pub/osm/node/123","name":"Louvre","latitude":48.8606,"longitude":2.3376},
		{"type":"Place","name":"Paris","latitude":48.8566,"longitude":2.3522},
		{"type":"Place","name":"Nowhere"},
		{"type":"Place","id":"https://places.pub/osm/node/123","name":"Louvre again","latitude":48.8606,"longitude":2.3376},
		{"type":"Place","id":"ftp://odd","name":"Odd","latitude":1,"longitude":2}
	]`)
	got := ReadPlaces(location, tag)
	if len(got) != 3 {
		t.Fatalf("read %+v; want Paris, the Louvre and Odd", got)
	}
	if got[0].Name != "Paris" || got[0].ID != "" || got[1].Name != "Louvre" || got[1].ID != "https://places.pub/osm/node/123" {
		t.Errorf("read %+v", got)
	}
	if got[2].ID != "" {
		t.Errorf("an address that is not https kept: %q", got[2].ID)
	}

	many := `[`
	for i := 0; i < 8; i++ {
		if i > 0 {
			many += ","
		}
		many += fmt.Sprintf(`{"type":"Place","name":"P%d","latitude":%d,"longitude":1}`, i, i)
	}
	many += `]`
	if got := ReadPlaces(nil, json.RawMessage(many)); len(got) != 5 {
		t.Errorf("read %d of eight; want five", len(got))
	}
	if got := ReadPlaces(nil, nil); len(got) != 0 {
		t.Errorf("read %+v from nothing", got)
	}
}
