package shell

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/linkpreviews"
)

// Reader reads what a page is about, as somebody who is not signed in.
// Each answers nil for what is not there or is not theirs to see.
type Reader interface {
	Person(alias string) (*entities.AnahitaNode, error)
	Actor(id int64, nodeType string) (*entities.AnahitaNode, error)
	Medium(id int64) (*entities.AnahitaNode, error)
}

// How much of a post or a bio is the description.
const maxDescription = 200

// Where a page's picture is asked for, on the shell itself: an address
// that stays the same, which a picture's signed address does not.
const ImagePath = "/_shell/image/"

// Subject is what a page is about: enough to write its tags, and to find
// its picture again when that is asked for.
type Subject struct {
	Tags Tags
	// The key of its picture in storage, or "" when it has none.
	ImageKey string
	// The name its picture is asked for by, under ImagePath.
	ImageName string
}

var (
	postPath  = regexp.MustCompile(`^/(notes|photos|videos|articles|topics)/(\d+)(?:-[^/]*)?/?$`)
	actorPath = regexp.MustCompile(`^/(groups|events)/(\d+)(?:-[^/]*)?(?:/.*)?$`)
	// Not a person's page: /people/add, and a person's settings and
	// notifications, which are theirs alone.
	personPath = regexp.MustCompile(`^/people/([^/]+)(?:/([^/]+))?(?:/.*)?$`)
	// /tags/<name> is what other servers are given for a hashtag's page:
	// see activitypub-service/ap/addresses.go. The web app sends it on.
	tagPath   = regexp.MustCompile(`^/(?:hashtags|tags)/([^/]+)/?$`)
	imageName = regexp.MustCompile(`^(person|group|event|post)-(\d+)$`)
)

// A group's type, which the shared constants do not name.
const groupType = "node.actor.group-service.group.v1"

var actorTypes = map[string]string{
	"groups": groupType,
	"events": constants.NodeActorEventEntityType,
}

var actorKinds = map[string]string{
	"groups": "group",
	"events": "event",
}

// Resolve finds what the page at a path is about. It answers nil for a
// page that is about nothing in particular, and for one about something
// that is not public: those are sent with what the site says of itself.
func Resolve(reader Reader, path string) *Subject {
	if found := postPath.FindStringSubmatch(path); found != nil {
		id, _ := strconv.ParseInt(found[2], 10, 64)
		return post(reader, id, found[1])
	}

	if found := actorPath.FindStringSubmatch(path); found != nil {
		if strings.Contains(path, "/settings") || strings.Contains(path, "/notifications") || strings.HasSuffix(path, "/edit") {
			return nil
		}
		id, _ := strconv.ParseInt(found[2], 10, 64)
		node, err := reader.Actor(id, actorTypes[found[1]])
		if err != nil {
			return nil
		}
		return actor(node, actorKinds[found[1]])
	}

	if found := personPath.FindStringSubmatch(path); found != nil {
		if found[1] == "add" || found[2] == "settings" || found[2] == "notifications" {
			return nil
		}
		node, err := reader.Person(found[1])
		if err != nil {
			return nil
		}
		return actor(node, "person")
	}

	if found := tagPath.FindStringSubmatch(path); found != nil {
		return &Subject{Tags: Tags{Title: "#" + found[1], Type: "website"}}
	}

	return nil
}

// ResolveImage finds the page whose picture is asked for by this name, by
// the same rules as the page itself: a picture is never easier to reach
// than what it is the picture of.
func ResolveImage(reader Reader, name string) *Subject {
	found := imageName.FindStringSubmatch(name)
	if found == nil {
		return nil
	}
	id, _ := strconv.ParseInt(found[2], 10, 64)

	switch found[1] {
	case "post":
		return post(reader, id, "")
	case "group":
		node, err := reader.Actor(id, groupType)
		if err != nil {
			return nil
		}
		return actor(node, "group")
	case "event":
		node, err := reader.Actor(id, constants.NodeActorEventEntityType)
		if err != nil {
			return nil
		}
		return actor(node, "event")
	default:
		node, err := reader.Actor(id, constants.NodeActorPersonEntityType)
		if err != nil {
			return nil
		}
		return actor(node, "person")
	}
}

// IsPublic says whether something may be described to anybody at all.
//
// The reads are already made as somebody who is not signed in. This is
// asked as well, of what came back, because a tag is copied by every site
// the link is pasted into and cannot be taken back: only what says of
// itself that it is public, on a profile that says the same, is described.
func IsPublic(node *entities.AnahitaNode) bool {
	if node == nil || node.ID == 0 || node.Restricted {
		return false
	}
	if node.DeletedAt != nil || node.ArchivedAt != nil {
		return false
	}
	if node.Access != constants.AnahitaAccessPublic {
		return false
	}
	if node.Owner != nil && node.Owner.ID != 0 && node.Owner.ID != node.ID && node.Owner.Access != constants.AnahitaAccessPublic {
		return false
	}
	return true
}

func actor(node *entities.AnahitaNode, kind string) *Subject {
	if !IsPublic(node) || !node.Enabled {
		return nil
	}

	subject := &Subject{
		Tags: Tags{
			Title:       strings.TrimSpace(node.Name),
			Description: Plain(node.Body, maxDescription),
			Type:        "profile",
			ImageAlt:    node.Name,
		},
		ImageName: kind + "-" + strconv.FormatInt(node.ID, 10),
	}
	if subject.Tags.Title == "" {
		subject.Tags.Title = node.Alias
	}
	subject.ImageKey = pathOf(node.Avatar, entities.AvatarSizeLarge)

	return subject
}

// The page a post is read at, by its kind of path. A post is described
// only at its own: /notes/12 for a note, and not for a photo with that id.
var postTypes = map[string]string{
	"notes":    "note",
	"photos":   "photo",
	"videos":   "video",
	"articles": "article",
	"topics":   "topic",
}

func post(reader Reader, id int64, kind string) *Subject {
	node, err := reader.Medium(id)
	if err != nil || !IsPublic(node) {
		return nil
	}
	if !strings.HasPrefix(node.Type, constants.NodeMediumPrefixEntityType) {
		return nil
	}
	if kind != "" && !strings.Contains(node.Type, "."+postTypes[kind]+".") {
		return nil
	}
	// A reply is read by whoever can read the post at the top of its
	// thread, so that is asked of that post too.
	if node.RootID > 0 && node.RootID != node.ID {
		root, err := reader.Medium(node.RootID)
		if err != nil || !IsPublic(root) {
			return nil
		}
	}

	author := ""
	if node.Author != nil {
		author = node.Author.Name
	}

	subject := &Subject{
		Tags: Tags{
			Title:       strings.TrimSpace(node.Name),
			Description: Plain(node.Body, maxDescription),
			Type:        "article",
			LargeImage:  true,
		},
		ImageName: "post-" + strconv.FormatInt(node.ID, 10),
	}
	if subject.Tags.Title == "" {
		// A note has no title: it is its writer's, and its words are the
		// description.
		subject.Tags.Title = author
	}
	if node.Excerpt != "" {
		subject.Tags.Description = Plain(node.Excerpt, maxDescription)
	}
	subject.Tags.ImageAlt = subject.Tags.Title

	// Its own picture: a photo, an article's cover, or the picture of the
	// link a note carries a card for.
	switch {
	case pathOf(node.Portrait, entities.PortraitSizeLarge) != "":
		subject.ImageKey = pathOf(node.Portrait, entities.PortraitSizeLarge)
	case pathOf(node.Cover, entities.CoverSizeLarge) != "":
		subject.ImageKey = pathOf(node.Cover, entities.CoverSizeLarge)
	default:
		if card := linkpreviews.FromMeta(node.Meta); card != nil && card.ImageKey != "" {
			subject.ImageKey = linkpreviews.StoragePrefix + card.ImageKey
		}
	}

	return subject
}

func pathOf(images map[string]entities.AnahitaNodeImage, size string) string {
	if image, ok := images[size]; ok {
		return image.Url
	}
	return ""
}

var (
	markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	// A heading's marks and a quote's, at the start of a line. A # inside a
	// line is a hashtag and stays.
	lineMarks  = regexp.MustCompile(`(?m)^\s*(?:#+|>+)\s+`)
	markup     = regexp.MustCompile("[*_`~]+")
	tagsInText = regexp.MustCompile(`<[^>]*>`)
	spaces     = regexp.MustCompile(`\s+`)
)

// Plain is a post's text as one plain line, cut to a length: Markdown and
// any HTML taken out, since a description is shown as it is written.
func Plain(text string, max int) string {
	text = tagsInText.ReplaceAllString(text, " ")
	text = markdownLink.ReplaceAllString(text, "$1")
	text = lineMarks.ReplaceAllString(text, "")
	text = markup.ReplaceAllString(text, "")
	text = strings.TrimSpace(spaces.ReplaceAllString(text, " "))

	if utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}
