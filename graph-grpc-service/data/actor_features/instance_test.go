package features

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/proto/graphpb"
	"anahita-libs/videos"
)

// A copy of the features directory as the image carries it, with the
// instance.json given, or none.
func sitesFeatures(t *testing.T, instance string) string {
	t.Helper()
	dir := t.TempDir()
	files, err := filepath.Glob("../../../graph-grpc-defaults/actor_features/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("the defaults: %v", err)
	}
	for _, file := range files {
		if filepath.Base(file) == InstanceFile {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(file)), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if instance != "" {
		if err := os.WriteFile(filepath.Join(dir, InstanceFile), []byte(instance), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const coffeehouse = `[
  { "service": "actor-service", "node_types": ["person", "group"] },
  { "service": "text-service",  "node_types": ["note"] },
  { "service": "photo-service", "node_types": ["photo"] },
  { "service": "video-service", "enabled": false }
]`

// What a person of the site has, its own saved choices given.
func personOf(t *testing.T, dir, saved string) *graphpb.Node {
	t.Helper()
	loader := NewFeatureLoader(dir)
	defaults, err := loader.Load(constants.NodeActorPersonEntityType)
	if err != nil {
		t.Fatal(err)
	}
	node := &graphpb.Node{Type: constants.NodeActorPersonEntityType, FeaturesRaw: saved}
	if err := ApplyFeatures(node, loader, defaults); err != nil {
		t.Fatal(err)
	}
	return node
}

func featureOf(node *graphpb.Node, service string) *graphpb.NodeFeature {
	for _, feature := range node.Features {
		if feature.Service == service {
			return feature
		}
	}
	return nil
}

// A profile has only what the instance offers, whatever it saved.
func TestAProfileHasOnlyWhatTheInstanceOffers(t *testing.T) {
	t.Setenv(videos.EnvUploads, "on")
	dir := sitesFeatures(t, coffeehouse)

	// It saved articles, and a video feature, before the site turned them off.
	person := personOf(t, dir, `[{"service":"text-service","node_types":["note","article"],"composers":["note","article"]},{"service":"video-service","enabled":true}]`)

	text := featureOf(person, "text-service")
	if text == nil || !slices.Equal(text.NodeTypes, []string{"note"}) || !slices.Equal(text.Composers, []string{"note"}) {
		t.Fatalf("text-service: %v", text)
	}
	for _, permission := range text.AddPermissions {
		if permission.Entity == "article" {
			t.Errorf("who may add an article is still said: %v", text.AddPermissions)
		}
	}
	if featureOf(person, videos.Service) != nil {
		t.Error("video-service is offered where the instance has it off")
	}
	if featureOf(person, "photo-service") == nil {
		t.Error("photos are gone")
	}
	// Who may tag or mention is not a kind of post, and stays.
	social := featureOf(person, "socialgraph-service")
	if social == nil || len(social.AddPermissions) == 0 {
		t.Errorf("socialgraph-service lost its permissions: %v", social)
	}
}

// No file is everything, as before.
func TestNoInstanceFileOffersEverything(t *testing.T) {
	t.Setenv(videos.EnvUploads, "on")
	t.Setenv(videos.EnvAudioUploads, "")
	dir := sitesFeatures(t, "")

	person := personOf(t, dir, "")
	if text := featureOf(person, "text-service"); text == nil || !slices.Contains(text.NodeTypes, "article") {
		t.Errorf("articles are gone with no instance.json: %v", text)
	}

	profiles, posts, err := NewFeatureLoader(dir).Offered()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(profiles, []string{"person", "group", "event", "website"}) {
		t.Errorf("profiles: %v", profiles)
	}
	for _, kind := range []string{"note", "article", "topic", "photo", "video", "audio"} {
		if !slices.Contains(posts, kind) {
			t.Errorf("%s is not offered: %v", kind, posts)
		}
	}
}

// What a coffeehouse offers, for NodeInfo.
func TestACoffeehouseOffersPeopleGroupsNotesAndPhotos(t *testing.T) {
	t.Setenv(videos.EnvUploads, "on")
	profiles, posts, err := NewFeatureLoader(sitesFeatures(t, coffeehouse)).Offered()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(profiles, []string{"person", "group"}) {
		t.Errorf("profiles: %v", profiles)
	}
	if !slices.Equal(posts, []string{"note", "photo"}) {
		t.Errorf("posts: %v", posts)
	}
}

// A person is an account, offered whatever the file says; and video stays
// off where VIDEO_UPLOADS is, whatever the file says.
func TestAPersonIsAlwaysOfferedAndVideoUploadsStillCounts(t *testing.T) {
	t.Setenv(videos.EnvUploads, "")
	profiles, posts, err := NewFeatureLoader(sitesFeatures(t, `[{"service":"actor-service","node_types":["group"]}]`)).Offered()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(profiles, "person") {
		t.Errorf("no person: %v", profiles)
	}
	if slices.Contains(posts, "video") {
		t.Errorf("video offered where VIDEO_UPLOADS is not on: %v", posts)
	}
}

// A file that cannot be read offers everything, and says so.
func TestABrokenInstanceFileOffersEverything(t *testing.T) {
	t.Setenv(videos.EnvUploads, "on")
	person := personOf(t, sitesFeatures(t, `[{"service": "text-service", "node_types": `), "")
	if text := featureOf(person, "text-service"); text == nil || !slices.Contains(text.NodeTypes, "article") {
		t.Errorf("a broken file took articles away: %v", text)
	}
	if _, err := LoadInstance(strings.Repeat("/no", 3)); err != nil {
		t.Errorf("no directory is no file: %v", err)
	}
}
