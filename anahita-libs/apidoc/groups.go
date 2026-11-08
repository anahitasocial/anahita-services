package apidoc

// Group is a heading of the reference with what is under it, in the order it
// is read: services, and the sections of one where it has them.
type Group struct {
	Name     string
	Services []string
}

// Groups are the reference's headings, the same as the docs' own
// (docs/README.md): anahita.json carries them as x-tagGroups, and the page
// lists what is under them. A tag in none of them is an error, so that a new
// one is put where it belongs.
var Groups = []Group{
	// actor-service's sections (actor-service/cmd/api/routes.go): what every
	// profile has, then each kind's own.
	{"Profiles", []string{
		"profiles",
		"people",
		"groups",
		"events",
		"websites",
	}},
	{"Accounts and the graph", []string{
		"auth-service",
		"socialgraph-service",
		"identity-image-service",
		"notification-service",
		"recommendation-service",
		"import-export-service",
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
