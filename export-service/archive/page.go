package archive

import (
	"archive/zip"
	"fmt"
	"html/template"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// pageData is everything the archive's own page shows.
type pageData struct {
	Site    string
	Made    time.Time
	Profile *entities.AnahitaNode
	URL     string
	// Paths in the archive, or "" where there is none.
	Avatar string
	Cover  string

	Posts     []*Post
	Following []*Ref
	Followers []*Ref
	Blocked   []*Ref
	Liked     []*Ref
	Saved     []*Ref
	Summary   *Summary
}

// Profile is the profile as it is in data/profile.json.
type Profile struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Alias     string    `json:"alias"`
	URL       string    `json:"url"`
	About     string    `json:"about,omitempty"`
	Pronouns  string    `json:"pronouns,omitempty"`
	Website   string    `json:"website,omitempty"`
	Language  string    `json:"language,omitempty"`
	Audience  string    `json:"audience,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
	Avatar    string    `json:"avatar,omitempty"`
	Cover     string    `json:"cover,omitempty"`
	Followers int64     `json:"followers"`
	Following int64     `json:"following"`
	// What their new posts start with.
	QuotePolicy   string    `json:"quote_policy,omitempty"`
	AllowDownload bool      `json:"allow_download"`
	ExportedAt    time.Time `json:"exported_at"`
}

func (b *Builder) profileOf(node *entities.AnahitaNode, page *pageData) *Profile {
	return &Profile{
		ID:            node.ID,
		Name:          node.Name,
		Alias:         node.Alias,
		URL:           page.URL,
		About:         node.Body,
		Pronouns:      node.PersonPronouns,
		Website:       node.WebsiteURL,
		Language:      node.Language,
		Audience:      node.Access,
		JoinedAt:      node.CreatedAt.UTC(),
		Avatar:        page.Avatar,
		Cover:         page.Cover,
		Followers:     node.FollowerCount,
		Following:     node.LeaderCount,
		QuotePolicy:   node.QuotePolicy,
		AllowDownload: node.AllowDownload,
		ExportedAt:    page.Made,
	}
}

const streams = "https://www.w3.org/ns/activitystreams"

// actorOf is the person as other fediverse software describes one.
func (b *Builder) actorOf(node *entities.AnahitaNode, page *pageData) map[string]any {
	actor := map[string]any{
		"@context":          streams,
		"id":                page.URL,
		"type":              "Person",
		"preferredUsername": node.Alias,
		"name":              node.Name,
		"summary":           node.Body,
		"url":               page.URL,
		"published":         node.CreatedAt.UTC().Format(time.RFC3339),
	}
	if page.Avatar != "" {
		actor["icon"] = map[string]any{"type": "Image", "url": page.Avatar}
	}
	if page.Cover != "" {
		actor["image"] = map[string]any{"type": "Image", "url": page.Cover}
	}
	return actor
}

// The kinds of object the things written are, in ActivityStreams.
var objectTypes = map[string]string{
	"note":    "Note",
	"reply":   "Note",
	"topic":   "Note",
	"article": "Article",
	"photo":   "Note",
	"video":   "Note",
}

// outboxOf is what was written, as the list of activities other fediverse
// software reads: a Create for each post, an Announce for each repost.
// Custom feeds are not activities and are left to data/posts.ndjson.
func (b *Builder) outboxOf(profile *entities.AnahitaNode, posts []*Post) map[string]any {
	actor := b.urlOf(profile)
	items := make([]map[string]any, 0, len(posts))

	for _, post := range posts {
		published := post.CreatedAt.Format(time.RFC3339)

		// Only what is public is addressed to everybody. Anything else is
		// addressed to nobody here: who could see it was this site's to say.
		to := []string{}
		if post.Audience == constants.AnahitaAccessPublic {
			to = append(to, streams+"#Public")
		}

		if post.RepostOf != nil {
			items = append(items, map[string]any{
				"id":        fmt.Sprintf("%s#reposts/%d", actor, post.ID),
				"type":      "Announce",
				"actor":     actor,
				"published": published,
				"to":        to,
				"object":    post.RepostOf.URL,
			})
			continue
		}

		kind, ok := objectTypes[post.Kind]
		if !ok {
			continue
		}

		object := map[string]any{
			"id":           post.URL,
			"type":         kind,
			"attributedTo": actor,
			"published":    published,
			"url":          post.URL,
			"to":           to,
			"content":      post.Text,
			"mediaType":    "text/markdown",
		}
		if post.Title != "" {
			object["name"] = post.Title
		}
		if post.InReplyTo > 0 {
			object["inReplyTo"] = fmt.Sprintf("%s/notes/%d", b.WebURL, post.InReplyTo)
		}
		if len(post.Media) > 0 {
			attachments := make([]map[string]any, 0, len(post.Media))
			for _, file := range post.Media {
				media := "image/jpeg"
				switch {
				case strings.HasSuffix(file, ".mp4"):
					media = "video/mp4"
				case strings.HasSuffix(file, ".png"):
					media = "image/png"
				}
				attachments = append(attachments, map[string]any{
					"type":      "Document",
					"mediaType": media,
					"url":       file,
				})
			}
			object["attachment"] = attachments
		}

		items = append(items, map[string]any{
			"id":        post.URL + "#create",
			"type":      "Create",
			"actor":     actor,
			"published": published,
			"to":        to,
			"object":    object,
		})
	}

	return map[string]any{
		"@context":     streams,
		"id":           "outbox.json",
		"type":         "OrderedCollection",
		"totalItems":   len(items),
		"orderedItems": items,
	}
}

func readme(page *pageData) string {
	name := page.Profile.Name
	if name == "" {
		name = page.Profile.Alias
	}
	return fmt.Sprintf(`%s's data from %s
Made %s

Open index.html in a browser to read it. Nothing in it is fetched from
anywhere: it works with no connection.

  index.html           everything here, as a page to read
  data/profile.json    the profile
  data/posts.ndjson    everything written, one post on each line: notes,
                       articles, topics, photos, videos, replies, reposts
                       and custom feeds
  data/following.json  who is followed
  data/followers.json  who follows
  data/blocked.json    who is blocked
  data/liked.json      what was liked
  data/saved.json      what was saved
  actor.json           the profile, as ActivityStreams
  outbox.json          the posts, as ActivityStreams: the form other
                       fediverse software reads
  media/               the pictures and videos posted, in a folder for each
                       post, and the profile's own pictures

%d posts and %d files. %s

What other people wrote is not here, apart from the names of what was
replied to, reposted, liked and saved. Posts that were removed are not here.
`, name, page.Site, page.Made.Format("2 January 2006, 15:04 UTC"), page.Summary.Posts, page.Summary.Files, missing(page.Summary))
}

func missing(summary *Summary) string {
	if summary.Missing == 0 {
		return ""
	}
	return fmt.Sprintf("%d files could not be read from storage and were left out.", summary.Missing)
}

// The page. Its own styles, no scripts, nothing from anywhere else.
// html/template escapes everything put into it: every name and every post
// is somebody's text.
var pageTemplate = template.Must(template.New("index").Funcs(template.FuncMap{
	"date": func(t time.Time) string {
		return t.Format("2 January 2006, 15:04")
	},
	"isVideo": func(path string) bool {
		return strings.HasSuffix(path, ".mp4")
	},
	"missing": missing,
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Profile.Name}} — {{.Site}}</title>
<style>
  body { font: 16px/1.5 system-ui, sans-serif; margin: 0; color: #1c1c1c; background: #f4f4f4; }
  main { max-width: 720px; margin: 0 auto; padding: 16px; }
  header img.cover { width: 100%; max-height: 240px; object-fit: cover; border-radius: 8px; }
  header img.avatar { width: 96px; height: 96px; border-radius: 50%; object-fit: cover; }
  h1 { margin: 8px 0 0; } h2 { margin: 32px 0 8px; }
  .muted { color: #666; font-size: 14px; }
  article, section.list { background: #fff; border-radius: 8px; padding: 16px; margin: 0 0 16px; }
  article img, article video { max-width: 100%; border-radius: 8px; margin-top: 8px; display: block; }
  article p { white-space: pre-wrap; overflow-wrap: anywhere; margin: 8px 0 0; }
  nav a { margin-right: 16px; }
  ul { margin: 0; padding-left: 24px; }
</style>
</head>
<body>
<main>
<header>
  {{if .Cover}}<img class="cover" src="{{.Cover}}" alt="">{{end}}
  {{if .Avatar}}<img class="avatar" src="{{.Avatar}}" alt="">{{end}}
  <h1 dir="auto">{{.Profile.Name}}</h1>
  <div class="muted">@{{.Profile.Alias}} · <a href="{{.URL}}">{{.URL}}</a></div>
  {{if .Profile.Body}}<p dir="auto">{{.Profile.Body}}</p>{{end}}
  <p class="muted">A copy of this profile's data from {{.Site}}, made {{date .Made}} UTC.
    {{.Summary.Posts}} posts and {{.Summary.Files}} files. {{missing .Summary}}</p>
  <nav>
    <a href="#posts">Posts</a><a href="#following">Following</a><a href="#followers">Followers</a>
    <a href="#liked">Liked</a><a href="#saved">Saved</a><a href="#blocked">Blocked</a>
  </nav>
</header>

<h2 id="posts">Posts ({{len .Posts}})</h2>
{{range .Posts}}
<article>
  <div class="muted">
    {{.Kind}} · {{date .CreatedAt}}{{if .Audience}} · {{.Audience}}{{end}}
    {{if .PostedOn}} · on {{.PostedOn.Name}}{{end}}
    {{if .URL}} · <a href="{{.URL}}">on the site</a>{{end}}
  </div>
  {{if .RepostOf}}<p>Reposted <a href="{{.RepostOf.URL}}">{{if .RepostOf.Name}}{{.RepostOf.Name}}{{else}}a post{{end}}</a></p>{{end}}
  {{if .Title}}<h3 dir="auto">{{.Title}}</h3>{{end}}
  {{if .Text}}<p dir="auto">{{.Text}}</p>{{end}}
  {{range .Media}}
    {{if isVideo .}}<video src="{{.}}" controls preload="none"></video>{{else}}<img src="{{.}}" alt="" loading="lazy">{{end}}
  {{end}}
</article>
{{else}}<p class="muted">Nothing posted.</p>{{end}}

{{define "refs"}}
<section class="list">
  {{if .}}<ul>{{range .}}<li><a href="{{.URL}}">{{if .Name}}{{.Name}}{{else if .Alias}}@{{.Alias}}{{else}}{{.Kind}} {{.ID}}{{end}}</a>{{if and .Name .Alias}} <span class="muted">@{{.Alias}}</span>{{end}}</li>{{end}}</ul>
  {{else}}<span class="muted">Nobody and nothing.</span>{{end}}
</section>
{{end}}

<h2 id="following">Following ({{len .Following}})</h2>{{template "refs" .Following}}
<h2 id="followers">Followers ({{len .Followers}})</h2>{{template "refs" .Followers}}
<h2 id="liked">Liked ({{len .Liked}})</h2>{{template "refs" .Liked}}
<h2 id="saved">Saved ({{len .Saved}})</h2>{{template "refs" .Saved}}
<h2 id="blocked">Blocked ({{len .Blocked}})</h2>{{template "refs" .Blocked}}
</main>
</body>
</html>
`))

func writePage(z *zip.Writer, page *pageData) error {
	out, err := z.Create("index.html")
	if err != nil {
		return err
	}
	return pageTemplate.Execute(out, page)
}
