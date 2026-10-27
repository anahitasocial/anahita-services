package apidoc

// Group is a heading of the reference with the services under it, in the
// order they are read.
type Group struct {
	Name     string
	Services []string
}

// Groups are the reference's headings, the same as the docs' own
// (docs/README.md): anahita.json carries them as x-tagGroups, and the page
// lists its services under them. A service in none of them is an error, so
// that a new one is put where it belongs.
var Groups = []Group{
	{"People", []string{
		"auth-service",
		"actor-service",
		"socialgraph-service",
		"identity-image-service",
		"notification-service",
		"recommendation-service",
		"export-service",
	}},
	{"Posts", []string{
		"text-service",
		"photo-service",
		"video-service",
		"like-service",
		"link-preview-service",
		"hashtag-service",
		"location-service",
		"feed-service",
		"search-service",
	}},
	{"Fediverse and Bluesky", []string{
		"activitypub-service",
		"atproto-service",
		"syndication-service",
	}},
	{"Run a site", []string{
		"abuse-report-service",
	}},
}

// places is each service's place in the reference, by its name.
func places() map[string]int {
	at := map[string]int{}
	for _, g := range Groups {
		for _, name := range g.Services {
			at[name] = len(at)
		}
	}
	return at
}
