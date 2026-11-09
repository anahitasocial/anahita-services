package features

import (
	"slices"
	"testing"

	"anahita-libs/data/entities"
	"anahita-libs/proto/graphpb"
	"anahita-libs/videos"
)

// Sound is the installation's to allow (AUDIO_UPLOADS). Where it is off,
// video-service's feature keeps its videos and loses its audio posts.
func TestAudioIsOnlyAFeatureWhereItIsAllowed(t *testing.T) {
	defaults := []*entities.AnahitaNodeFeature{
		{
			Service: videos.Service, Enabled: true,
			NodeTypes: []string{"video", "audio"}, Composers: []string{"video", "audio"},
			AddPermissions: []entities.AnahitaNodeAddPermission{{Entity: "video", Access: "followers"}, {Entity: "audio", Access: "followers"}},
		},
	}
	loader := NewFeatureLoader(t.TempDir())
	t.Setenv(videos.EnvUploads, "on")

	t.Setenv(videos.EnvAudioUploads, "off")
	off := &graphpb.Node{}
	if err := ApplyFeatures(off, loader, defaults); err != nil {
		t.Fatal(err)
	}
	feature := off.Features[0]
	if slices.Contains(feature.NodeTypes, "audio") || slices.Contains(feature.Composers, "audio") || len(feature.AddPermissions) != 1 {
		t.Errorf("audio is a feature where it is off: %+v", feature)
	}
	if !slices.Contains(feature.NodeTypes, "video") || !slices.Contains(feature.Composers, "video") {
		t.Errorf("video went with audio: %+v", feature)
	}

	t.Setenv(videos.EnvAudioUploads, "")
	on := &graphpb.Node{}
	if err := ApplyFeatures(on, loader, defaults); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(on.Features[0].Composers, "audio") {
		t.Errorf("audio is not a feature where it is allowed: %+v", on.Features[0])
	}
	// The defaults themselves are left as they were.
	if !slices.Contains(defaults[0].Composers, "audio") {
		t.Error("the defaults lost their audio")
	}
}
