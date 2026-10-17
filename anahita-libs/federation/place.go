package federation

import (
	"encoding/json"
	"strings"
)

// Place is a location as ActivityStreams 2.0 has it: the `location` of an
// object, a `Place` with a `latitude` and a `longitude`, and perhaps a
// `name` (https://www.w3.org/TR/activitystreams-vocabulary/#dfn-place).
// Pixelfed sends one with a post its author chose a place for, and Mastodon
// none.
//
// Only the reading and the writing of it, and nothing that calls either
// (the user, 2026-10-17): nobody's location is taken from what they post,
// and none is sent, until how people consent to it is decided (roadmap row
// 82). Devices and animal trackers that post their position are what it is
// kept for.
type Place struct {
	Name      string
	Latitude  float64
	Longitude float64
}

// The longest name kept of a place.
const placeNameMax = 100

// ReadPlace is the place a `location` says, when it is a Place with a point
// on the Earth: the first such, when it lists several. Nil otherwise: a
// place with no coordinates, or a Link, or anything else, is no place here.
// `country` and other keys that are not the vocabulary's are not read.
func ReadPlace(location json.RawMessage) *Place {
	trimmed := strings.TrimSpace(string(location))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	var many []json.RawMessage
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(location, &many); err != nil {
			return nil
		}
	} else {
		many = []json.RawMessage{location}
	}

	for _, one := range many {
		var shape struct {
			Type      any      `json:"type"`
			Name      string   `json:"name"`
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
		}
		if err := json.Unmarshal(one, &shape); err != nil {
			continue
		}
		if !typeIs(shape.Type, "Place") || shape.Latitude == nil || shape.Longitude == nil {
			continue
		}
		lat, lon := *shape.Latitude, *shape.Longitude
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			continue
		}
		name := strings.TrimSpace(shape.Name)
		if letters := []rune(name); len(letters) > placeNameMax {
			name = string(letters[:placeNameMax])
		}
		return &Place{Name: name, Latitude: lat, Longitude: lon}
	}
	return nil
}

// AsLocation is the place as an object's `location`, for sending.
func (p Place) AsLocation() map[string]any {
	location := map[string]any{
		"type":      "Place",
		"latitude":  p.Latitude,
		"longitude": p.Longitude,
	}
	if p.Name != "" {
		location["name"] = p.Name
	}
	return location
}

// typeIs says whether an ActivityStreams `type`, one name or several, is
// this one.
func typeIs(value any, want string) bool {
	switch kind := value.(type) {
	case string:
		return kind == want
	case []any:
		for _, each := range kind {
			if name, ok := each.(string); ok && name == want {
				return true
			}
		}
	}
	return false
}
