package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"activitypub/ap"
	"activitypub/data/repositories"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"

	"anahita-libs/videos"
	"github.com/labstack/echo/v4"
	"html"
	"regexp"
)

// Posts going out.
//
// WHAT GOES OUT CANNOT BE CALLED BACK, so the rule for what does is narrow
// and is asked every time, of the post as it is now:
//
//   - a note or a photo post, written here, by somebody who is on the
//     fediverse. A photo post is sent as a note with its pictures;
//   - on their own profile, not in a group and not on somebody else's;
//   - open to everybody: not to members, not to followers;
//   - or a reply in a thread that is open to everybody, to something other
//     servers can see: see answers;
//   - not hidden, and not removed;
//   - and a visitor may see it, by the graph's own reckoning.
//
// A post that stops being all of these after it went out is taken back from
// the servers that have it, as far as they will.

// goesOut says whether a post is one that other servers are told of, and
// who wrote it; and for a reply, what it answers.
func (h *Handlers) goesOut(note *entities.AnahitaNode) (*entities.AnahitaNode, *answer) {
	if note == nil || note.ID == 0 || note.UUID == "" || note.IsRemote() {
		return nil, nil
	}
	// A note, or a photo or video post, which other servers are sent as a
	// note with its pictures or its video. A video post with no video yet
	// is not one.
	isPhoto := note.Type == constants.NodeMediumPhotoEntityType
	isVideo := note.Type == videos.NodeType && videos.FromMeta(note.Meta) != nil
	// An article, as an Article. A topic stays here (the user, 2026-10-17).
	isArticle := note.Type == constants.NodeMediumArticleEntityType
	if (note.Type != constants.NodeMediumNoteEntityType && !isPhoto && !isVideo && !isArticle) || !note.Enabled || note.DeletedAt != nil || note.Hidden {
		return nil, nil
	}
	// In its author's media library only: shown within an article here,
	// and announced to nobody.
	if note.LibraryOnly {
		return nil, nil
	}
	// Somebody's own post, on their own profile; a website's, made of its
	// feed, is the website's on both counts.
	if note.CreatedBy <= 0 || note.OwnerID != note.CreatedBy {
		return nil, nil
	}

	// A reply says nothing of its own about who may read it: its thread
	// does. And it goes out only when what it answers is something other
	// servers can see.
	var answering *answer
	if note.ParentID > 0 || note.RootID > 0 {
		if answering = h.answers(note); answering == nil {
			return nil, nil
		}
	} else if note.Access != constants.AnahitaAccessPublic {
		return nil, nil
	}

	author, err := h.Nodes.Read(note.CreatedBy)
	if err != nil || h.federating(author) == nil {
		return nil, nil
	}
	if !permissions.CanReadByID(h.Nodes, note.ID, entities.NewViewer()) {
		return nil, nil
	}
	return author, answering
}

// answer is what a reply answers, as it is known between servers.
type answer struct {
	// The address of what is answered.
	ID string
	// Whoever wrote it, when they are on another server: their address,
	// what they are called, and where they are written to.
	Actor  string
	Handle string
	Inbox  string
}

// answers finds what a reply answers, or nil when the reply is not one
// that goes out: its thread is not open to everybody, or what it answers
// is nothing other servers have.
//
// What other servers have is a note that came from one of them, or a note
// here that itself went out. So a thread of somebody's own notes goes out
// whole, each on the heels of the one before, and a reply under a note
// that stayed here stays here too.
func (h *Handlers) answers(reply *entities.AnahitaNode) *answer {
	if reply.ParentID <= 0 || reply.RootID <= 0 {
		return nil
	}

	root, err := h.Nodes.Read(reply.RootID)
	if err != nil || root == nil || root.ID == 0 || !root.Enabled || root.Access != constants.AnahitaAccessPublic {
		return nil
	}

	parent := root
	if reply.ParentID != reply.RootID {
		if parent, err = h.Nodes.Read(reply.ParentID); err != nil || parent == nil || parent.ID == 0 {
			return nil
		}
	}
	if !parent.Enabled || parent.DeletedAt != nil || !isPost(parent) {
		return nil
	}

	if !parent.IsRemote() {
		sent, err := h.Objects.Read(parent.ID)
		if err != nil || sent == nil {
			return nil
		}
		return &answer{ID: h.addresses.Note(sent.UUID)}
	}

	// From another server: whoever wrote it is told.
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(parent.URI)) {
		return nil
	}
	kept, err := h.Actors.ReadByNode(parent.OwnerID)
	if err != nil || kept == nil {
		return nil
	}
	writer, err := h.Nodes.Read(parent.OwnerID)
	if err != nil || writer == nil || writer.Alias == "" {
		return nil
	}

	inbox := kept.SharedInbox
	if inbox == "" {
		inbox = kept.Inbox
	}
	return &answer{ID: parent.URI, Actor: kept.URI, Handle: "@" + writer.Alias, Inbox: inbox}
}

// How long a photo post's pictures are waited for before it is left to the
// sweep. A variable, so that tests do not wait.
var picturesWait = 2 * time.Second

func (h *Handlers) hasPictures(postID int64) bool {
	files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{postID})
	return err == nil && len(files) > 0
}

// Locations reads what a post is tied to in the graph: here, the locations
// it was tagged with.
type Locations interface {
	BrowseConnectionNodes(req *services.GraphBrowseConnectionNodesRequest) ([]*entities.AnahitaNode, uint64, error)
}

// placesOf is the locations a post was tagged with, the first chosen first,
// as the Places it goes out with (roadmap row 82, part 3b). Read as a super
// administrator would, the post having been judged to go out already.
func (h *Handlers) placesOf(post *entities.AnahitaNode) []federation.Place {
	if h.Locations == nil {
		return nil
	}
	viewer := entities.NewViewer()
	viewer.PersonType = constants.PersonTypeSuperAdmin
	found, _, err := h.Locations.BrowseConnectionNodes(&services.GraphBrowseConnectionNodesRequest{
		NodeId:      post.ID,
		EdgeType:    constants.EdgeTagLocationType,
		Direction:   constants.GraphQueryDirectionInbound,
		Hops:        1,
		Limit:       uint64(constants.MaxLocationsOfAPost),
		Ordering:    constants.OrderingConnected,
		OrderingDir: constants.DirAsc,
		Viewer:      viewer,
	})
	if err != nil {
		log.Printf("post %d: reading its locations: %v", post.ID, err)
		return nil
	}
	places := []federation.Place{}
	for _, location := range found {
		if location == nil || location.Type != constants.NodeBaseLocationEntityType {
			continue
		}
		if location.GeoLatitude == 0 && location.GeoLongitude == 0 {
			continue
		}
		places = append(places, federation.Place{
			Name:      location.Name,
			Latitude:  location.GeoLatitude,
			Longitude: location.GeoLongitude,
			URL:       h.addresses.LocationPage(location.ID, location.Alias),
		})
	}
	return places
}

// described is a note as other servers are sent it, a reply with what it
// answers.
func (h *Handlers) described(note, author *entities.AnahitaNode, answering *answer) *ap.Note {
	sent := ap.NewNote(h.addresses, note, author, h.links())
	if note.Type == constants.NodeMediumArticleEntityType {
		sent.AsArticle(h.addresses, note, articleSummary(note, h.links()))
	}
	if answering != nil {
		sent.Answering(answering.ID, answering.Actor, answering.Handle)
	}
	// Where it is, as its author chose: the first as its location, all
	// in its tags.
	if places := h.placesOf(note); len(places) > 0 {
		sent.WithPlaces(places)
	}
	// A photo post goes with its pictures, each at an address of this
	// installation's that leads to it for as long as the post goes out.
	if note.Type == constants.NodeMediumPhotoEntityType && h.PhotoFiles != nil {
		if files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{note.ID}); err == nil {
			sent.WithPictures(h.addresses, note, files)
		}
	}
	// A video post goes with its video, at a size other servers take, and
	// its title above its words, since a note has no title of its own.
	if note.Type == videos.NodeType {
		if video := videos.FromMeta(note.Meta); video != nil {
			sent.WithVideo(h.addresses, note, video, video.Shared(ap.VideoMax))
		}
		if title := strings.TrimSpace(note.Name); title != "" {
			sent.Content = "<p>" + html.EscapeString(title) + "</p>" + sent.Content
		}
	}
	return sent
}

// Video sends a server that asks for a video post's file to where it is,
// and VideoPoster to its poster: signed addresses in storage, good for a
// short while. Only of a post that goes out, asked each time, as a picture
// is.
func (h *Handlers) Video(c echo.Context) error {
	return h.videoFile(c, func(video *videos.Video) string {
		if file := video.Shared(ap.VideoMax); file != nil {
			return file.Key
		}
		return ""
	})
}

func (h *Handlers) VideoPoster(c echo.Context) error {
	return h.videoFile(c, func(video *videos.Video) string {
		return video.Poster
	})
}

func (h *Handlers) videoFile(c echo.Context, keyOf func(*videos.Video) string) error {
	uuid := c.Param("uuid")
	if !uuidShape.MatchString(uuid) || h.Files == nil {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}
	post, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || post == nil || post.Type != videos.NodeType {
		return c.NoContent(http.StatusNotFound)
	}
	if author, _ := h.goesOut(post); author == nil {
		return c.NoContent(http.StatusNotFound)
	}
	key := keyOf(videos.FromMeta(post.Meta))
	if key == "" {
		return c.NoContent(http.StatusNotFound)
	}

	request := services.NewFileRequest().Presign
	request.Path = key
	address, err := h.Files.Presign(request)
	if err != nil || address == "" {
		return c.NoContent(http.StatusNotFound)
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return c.Redirect(http.StatusFound, address)
}

// Picture sends a server that asks for one of a photo post's pictures to
// where it is: a signed address in storage, good for a short while. Only a
// picture of a post that goes out, which is asked each time, so that a
// post closed to its followers stops giving its pictures to anybody.
func (h *Handlers) Picture(c echo.Context) error {
	uuid := c.Param("uuid")
	position, err := strconv.Atoi(c.Param("position"))
	// Up to the most any installation allows: a post made before the limit
	// here was lowered keeps its pictures.
	if !uuidShape.MatchString(uuid) || err != nil || position < 0 || position >= constants.PhotoMaxFilesCeiling {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok || h.PhotoFiles == nil || h.Files == nil {
		return c.NoContent(http.StatusNotFound)
	}

	post, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || post == nil || post.Type != constants.NodeMediumPhotoEntityType {
		return c.NoContent(http.StatusNotFound)
	}
	if author, _ := h.goesOut(post); author == nil {
		return c.NoContent(http.StatusNotFound)
	}

	files, err := h.PhotoFiles.BrowsePhotoFiles([]int64{post.ID})
	if err != nil || position >= len(files) {
		return c.NoContent(http.StatusNotFound)
	}
	sizes, err := post.PortraitsFor(files[position].Filename)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	// The largest copy kept: other servers make their own smaller ones.
	stored, ok := sizes[entities.PortraitSizeOriginal]
	if !ok || stored.Url == "" {
		return c.NoContent(http.StatusNotFound)
	}

	request := services.NewFileRequest().Presign
	request.Path = stored.Url
	address, err := h.Files.Presign(request)
	if err != nil || address == "" {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return c.Redirect(http.StatusFound, address)
}

// toReaders queues something about a post for everybody it is for: its
// author's followers and, for a reply, whoever it answers.
func (h *Handlers) toReaders(ctx context.Context, author *entities.AnahitaNode, replyInbox string, activity any, what string) error {
	if err := h.toFollowers(ctx, author, activity, what); err != nil {
		return err
	}
	if replyInbox == "" {
		return nil
	}
	// The same inbox twice is one row in the queue: a follower's server
	// that is also the server of whoever is answered is sent it once.
	return h.send(ctx, author.ID, replyInbox, activity, what)
}

// links says where the people and hashtags a post names are, for the links
// made of them. Somebody here is linked only when they are on the
// fediverse; somebody elsewhere, only when they are known here.
func (h *Handlers) links() federation.Links {
	return federation.Links{
		Hashtag: h.addresses.Hashtag,
		Mention: func(user, host string) string {
			if host == "" || h.isOurDomain(host) {
				if person := h.federating(h.personByAlias(user)); person != nil {
					return h.addresses.Actor(person.UUID)
				}
				return ""
			}
			req := services.NewGraphNodeRequest().ReadByAlias
			req.Alias = strings.ToLower(user) + "@" + host
			req.NodeType = constants.NodeActorActivityPubPersonEntityType
			if known, err := h.Nodes.ReadByAlias(req); err == nil && known != nil && known.URI != "" {
				return known.URI
			}
			return ""
		},
	}
}

// Note serves a post made here to a server that asks for it.
func (h *Handlers) Note(c echo.Context) error {
	uuid := c.Param("uuid")
	if !uuidShape.MatchString(uuid) {
		return c.NoContent(http.StatusNotFound)
	}
	if ok, _ := h.siteFederates(); !ok {
		return c.NoContent(http.StatusNotFound)
	}

	note, err := h.Nodes.ReadByUUID(uuid)
	if err != nil || note == nil {
		return c.NoContent(http.StatusNotFound)
	}
	author, answering := h.goesOut(note)
	if author == nil {
		return c.NoContent(http.StatusNotFound)
	}

	c.Response().Header().Set("Cache-Control", "public, max-age=180")
	return activityJSON(c, h.described(note, author, answering).WithContext())
}

// Outbox says how many posts a person has sent out, and not which. A server
// that follows them is sent each as it is made.
func (h *Handlers) Outbox(c echo.Context) error {
	person := h.actorOf(c)
	if person == nil {
		return c.NoContent(http.StatusNotFound)
	}
	count, err := h.Objects.Count(person.ID)
	if err != nil {
		return c.NoContent(http.StatusServiceUnavailable)
	}
	return activityJSON(c, ap.NewCollection(h.addresses.Outbox(person.UUID), count))
}

// toFollowers queues something for every server that has a follower of a
// person here.
func (h *Handlers) toFollowers(ctx context.Context, author *entities.AnahitaNode, activity any, what string) error {
	inboxes, err := h.Follows.FollowerInboxes(author.ID)
	if err != nil {
		return err
	}
	for _, inbox := range inboxes {
		if err := h.send(ctx, author.ID, inbox, activity, what); err != nil {
			return err
		}
	}
	return nil
}

// posted is a post having been made here, or changed: other servers are
// told, or told to drop it, by what it is now.
func (h *Handlers) posted(ctx context.Context, nodeID int64) error {
	return h.tellOf(ctx, nodeID, false)
}

// relocated is a location added to a post or taken off it, from its page:
// the post itself says nothing changed, so it is sent again as an Update
// with the time of this change (roadmap row 82, part 3b). One that never
// went out is sent as it would have been.
func (h *Handlers) relocated(ctx context.Context, nodeID int64) error {
	return h.tellOf(ctx, nodeID, true)
}

// tellOf tells other servers of a post as it is now: made, changed, or, with
// `retagged`, given other locations.
func (h *Handlers) tellOf(ctx context.Context, nodeID int64, retagged bool) error {
	note, err := h.Nodes.Read(nodeID)
	if err != nil {
		return err
	}
	if note == nil || note.ID == 0 {
		return h.removed(ctx, nodeID)
	}

	author, answering := h.goesOut(note)
	if author == nil {
		// Not one that goes out. If it did before (it was open to
		// everybody and is not now), it is taken back.
		return h.removed(ctx, nodeID)
	}

	// A photo post is heard of the moment it is made, a moment before its
	// pictures are listed. It is not sent without them: it is looked at
	// once more, and failing that is left for the sweep, which finds a post
	// that should have gone out and did not.
	if note.Type == constants.NodeMediumPhotoEntityType && h.PhotoFiles != nil {
		if !h.hasPictures(note.ID) {
			time.Sleep(picturesWait)
			if !h.hasPictures(note.ID) {
				return nil
			}
		}
	}

	sent := h.described(note, author, answering)
	replyInbox := ""
	if answering != nil {
		replyInbox = answering.Inbox
	}

	begun, err := h.Objects.Begin(&repositories.Object{NodeID: note.ID, UUID: note.UUID, ActorID: author.ID, ReplyInbox: replyInbox})
	if err != nil {
		return err
	}
	if begun {
		if err := h.toReaders(ctx, author, replyInbox, ap.NewCreate(h.addresses, sent, note.UUID), "create:"+note.UUID); err != nil {
			return err
		}
		return h.Objects.Told(note.ID, note.UpdatedAt)
	}

	// Known already: this is a change to it. Only when it says it was
	// changed, so that hearing of its making twice sends nothing twice; or
	// when its locations changed, which its own dates do not show, said as
	// changed now and sent once for each such change.
	key := fmt.Sprintf("update:%s:%s", note.UUID, sent.Updated)
	if retagged {
		now := time.Now().UTC()
		sent.Updated = now.Format(time.RFC3339)
		key = fmt.Sprintf("update:%s:locations:%d", note.UUID, now.UnixNano())
	}
	if sent.Updated == "" {
		return nil
	}
	if err := h.toReaders(ctx, author, replyInbox, ap.NewUpdate(h.addresses.Activity(newUUID()), sent), key); err != nil {
		return err
	}
	// What other servers now have, so that the sweep can tell a change
	// they were never sent from one they were.
	return h.Objects.Told(note.ID, note.UpdatedAt)
}

// removed is a post having gone, or having stopped being one that goes out:
// the servers that were told of it are told to drop it. A post they were
// never told of is nothing to them, and nothing is sent.
func (h *Handlers) removed(ctx context.Context, nodeID int64) error {
	object, err := h.Objects.Read(nodeID)
	if err != nil || object == nil {
		return err
	}
	// A repost is taken back by undoing it, not by saying it is gone.
	if object.Kind == repositories.KindRepost {
		return h.unreposted(ctx, object)
	}

	ended, err := h.Objects.End(nodeID)
	if err != nil || !ended {
		return err
	}

	author, err := h.Nodes.Read(object.ActorID)
	if err != nil || author == nil || author.UUID == "" {
		return err
	}
	return h.toReaders(ctx, author, object.ReplyInbox,
		ap.NewDelete(h.addresses.Activity(newUUID()), h.addresses.Actor(author.UUID), h.addresses.Note(object.UUID)),
		"delete:"+object.UUID)
}

// articleSummary is an article summed up for other servers: its first
// paragraph of words, as plain text, cut at a word near 300 characters.
// A heading, a picture, a quotation, a list or an address on a line of its
// own is passed over. Nothing when the article has no such paragraph.
func articleSummary(article *entities.AnahitaNode, links federation.Links) string {
	for _, paragraph := range strings.Split(strings.ReplaceAll(article.Body, "\r\n", "\n"), "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" || strings.ContainsAny(paragraph[:1], "#!>|-*<`") ||
			strings.HasPrefix(paragraph, "http://") || strings.HasPrefix(paragraph, "https://") {
			continue
		}
		words := strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(federation.ToHTML(paragraph, links).HTML, " "))), " ")
		if words == "" {
			continue
		}
		if letters := []rune(words); len(letters) > summaryMax {
			cut := string(letters[:summaryMax])
			if at := strings.LastIndex(cut, " "); at > summaryMax/2 {
				cut = cut[:at]
			}
			words = strings.TrimRight(cut, " ,;:.") + "…"
		}
		return html.EscapeString(words)
	}
	return ""
}

// The longest an article's summary is, and the marks taken out of it.
const summaryMax = 300

var tags = regexp.MustCompile(`<[^>]*>`)
