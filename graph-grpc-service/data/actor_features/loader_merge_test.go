package features

import (
	"slices"
	"testing"

	"anahita-libs/data/entities"
)

// THE RULE THIS GUARDS (roadmap row 137): a profile that saved its features
// keeps its own choices, and gains the kinds of post added to the defaults
// since, with who may add them as the defaults say.
func TestASavedFeatureGainsNewKinds(t *testing.T) {
	fl := &FeatureLoader{}
	defaults := []*entities.AnahitaNodeFeature{{
		Service:   "video-service",
		NodeTypes: []string{"video", "audio"},
		Composers: []string{"video", "audio"},
		Enabled:   true,
		AddPermissions: []entities.AnahitaNodeAddPermission{
			{Entity: "video", Access: "followers"},
			{Entity: "audio", Access: "followers"},
		},
	}}
	off := false
	saved := []*FeatureOverride{{
		Service:        "video-service",
		NodeTypes:      []string{"video"},
		Composers:      []string{"video"},
		Enabled:        &off,
		AddPermissions: []entities.AnahitaNodeAddPermission{{Entity: "video", Access: "admins"}},
	}}

	got := fl.Merge(defaults, saved)[0]
	if !slices.Equal(got.NodeTypes, []string{"video", "audio"}) || !slices.Equal(got.Composers, []string{"video", "audio"}) {
		t.Errorf("kinds: %v %v", got.NodeTypes, got.Composers)
	}
	if got.Enabled {
		t.Errorf("the profile's own choice to turn it off was lost")
	}
	want := []entities.AnahitaNodeAddPermission{{Entity: "video", Access: "admins"}, {Entity: "audio", Access: "followers"}}
	if !slices.Equal(got.AddPermissions, want) {
		t.Errorf("permissions: %v", got.AddPermissions)
	}
}
