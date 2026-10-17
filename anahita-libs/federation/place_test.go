package federation

import (
	"encoding/json"
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
