package features

import (
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"anahita-libs/videos"
)

// Video is the installation's to switch on. Where it is off, no profile
// has it, whatever the profile's own settings say.
func TestVideoIsOnlyAFeatureWhereItIsSwitchedOn(t *testing.T) {
	defaults := []*entities.AnahitaNodeFeature{
		{Service: "photo-service", Enabled: true, NodeTypes: []string{"photo"}},
		{Service: videos.Service, Enabled: true, NodeTypes: []string{"video"}, Composers: []string{"video"}},
	}
	has := func(node *graphpb.Node) bool {
		for _, feature := range node.Features {
			if feature.Service == videos.Service {
				return true
			}
		}
		return false
	}
	loader := NewFeatureLoader(t.TempDir())

	t.Setenv(videos.EnvUploads, "")
	off := &graphpb.Node{FeaturesRaw: `[{"service":"video-service","enabled":true}]`}
	if err := ApplyFeatures(off, loader, defaults); err != nil {
		t.Fatal(err)
	}
	if has(off) || len(off.Features) != 1 {
		t.Errorf("video is a feature where it is off: %v", off.Features)
	}

	t.Setenv(videos.EnvUploads, "on")
	on := &graphpb.Node{}
	if err := ApplyFeatures(on, loader, defaults); err != nil {
		t.Fatal(err)
	}
	if !has(on) || len(on.Features) != 2 {
		t.Errorf("video is not a feature where it is on: %v", on.Features)
	}
}
