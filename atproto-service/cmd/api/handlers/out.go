package handlers

import (
	libhandlers "anahita-libs/cmd/api/handlers"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"sync"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"
	"anahita-libs/linkpreviews"
	images "anahita-libs/utils"
	"anahita-libs/videos"
	"io"
	"net/http"
	"strings"
)

// What somebody here does to something from Bluesky, done there too.
//
// Only for somebody who connected their Bluesky account (settings.go). A
// like or a repost of a post that came from Bluesky, and a reply under
// one, or under a reply that itself went there. And a note or a photo
// post of their own that is open to everybody, on their own profile,
// which is posted there as it is here, pictures and all: connecting an account is saying so, as being on the
// fediverse is. All of it is written as their account, to the server that
// holds it.
//
// What is to be written waits in a table and is tried until it is done,
// as what goes to the fediverse does. Each thing is given its name before
// it is written and remembered by it: so that writing it twice writes it
// once, and so that when Bluesky tells of it, as it tells of everything
// the account does, it is known for this installation's own and is not
// kept a second time as something from elsewhere.

// Writer is what writes to an account's server.
type Writer interface {
	PDSOf(ctx context.Context, directory, did string) (string, error)
	CreateSession(ctx context.Context, pds, identifier, password string) (*bsky.Session, error)
	RefreshSession(ctx context.Context, pds, refreshJWT string) (*bsky.Session, error)
	CreateRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string, record any) error
	DeleteRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string) error
	UploadBlob(ctx context.Context, pds, accessJWT string, content []byte, mimeType string) (*bsky.OutBlob, error)
	Follows(ctx context.Context, pds, accessJWT, did string) (bool, error)
	Notifications(ctx context.Context, pds, accessJWT string, limit int) ([]bsky.Notification, error)
	RefOf(ctx context.Context, uri string) (post, root *bsky.Ref, err error)
}

// Enqueuer is the table of what waits to be written.
type Enqueuer interface {
	Enqueue(ctx context.Context, actorID int64, target string, payload []byte, dedupeKey string) error
}

// write is one thing waiting to be written.
type write struct {
	// "create" or "delete".
	Op         string `json:"op"`
	Collection string `json:"collection"`
	RKey       string `json:"rkey"`
	// For a like or a repost, or a reply: the post it is of, or under.
	Subject string `json:"subject,omitempty"`
	// For a reply: the node that holds its words.
	NodeID int64 `json:"node_id,omitempty"`
}

var collections = map[string]string{
	repositories.SentPost:   bsky.PostCollection,
	repositories.SentLike:   "app.bsky.feed.like",
	repositories.SentRepost: bsky.RepostCollection,
	repositories.SentFollow: bsky.FollowCollection,
}

// onBluesky is the address on Bluesky of something here: what it came
// with, when it came from there, or what it was written as, when it is a
// reply that went there. "" for anything else.
func (h *Handlers) onBluesky(node *entities.AnahitaNode) string {
	if node == nil || node.ID == 0 {
		return ""
	}
	if node.IsRemote() {
		if _, _, ok := bsky.ParsePostURI(node.URI); ok {
			return node.URI
		}
		return ""
	}
	sent, err := h.Sent.Read(repositories.SentPost, node.ID, 0)
	if err != nil || sent == nil {
		return ""
	}
	return bsky.PostURI(sent.DID, sent.RKey)
}

// connected is the account of somebody here that can be written as, or
// nil.
func (h *Handlers) connected(personID int64) *repositories.Account {
	if !h.canWrite() || personID <= 0 {
		return nil
	}
	account, err := h.Accounts.Read(personID)
	if err != nil || account == nil || account.BrokenAt != nil {
		return nil
	}
	return account
}

// begin remembers something about to be written and puts it in the queue.
func (h *Handlers) begin(ctx context.Context, account *repositories.Account, kind string, localID int64, subject string, nodeID int64) error {
	// Only for somebody whose profile anybody may see, as on the
	// fediverse: a profile kept to followers or to members writes nothing
	// to Bluesky, which is read by anybody.
	if !libhandlers.MaySee(h.Nodes, account.PersonID, entities.NewViewer()) {
		return nil
	}
	sent := &repositories.Sent{
		Kind: kind, PersonID: account.PersonID, LocalID: localID, DID: account.DID, RKey: bsky.NewTID(),
	}
	begun, err := h.Sent.Begin(sent)
	if err != nil || !begun {
		// Written already, or being written: another copy of this
		// service heard it first, or it is a reply that was changed.
		return err
	}
	return h.enqueue(ctx, account.PersonID, &write{
		Op: "create", Collection: collections[kind], RKey: sent.RKey, Subject: subject, NodeID: nodeID,
	})
}

// takeBack forgets something that was written and has it removed.
func (h *Handlers) takeBack(ctx context.Context, kind string, localID, personID int64) error {
	sent, err := h.Sent.Read(kind, localID, personID)
	if err != nil || sent == nil {
		return err
	}
	if err := h.Sent.Delete(sent.ID); err != nil {
		return err
	}
	return h.enqueue(ctx, sent.PersonID, &write{Op: "delete", Collection: collections[kind], RKey: sent.RKey})
}

func (h *Handlers) enqueue(ctx context.Context, personID int64, w *write) error {
	payload, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return h.Queue.Enqueue(ctx, personID, "bluesky", payload, fmt.Sprintf("%s:%s:%s", w.Op, w.Collection, w.RKey))
}

// A follow of somebody on Bluesky, by somebody here who connected their
// account, is their account following that one: there is no other kind of
// follow on Bluesky. Whoever is followed is told, and whoever follows is
// among their followers.
//
// An account that follows them already is left as it is: nothing is
// written, and nothing is remembered, so that unfollowing here later does
// not take away a follow that was made on Bluesky. Only what this
// installation wrote is taken back by it.

// localFollowed is somebody here having followed somebody on Bluesky.
func (h *Handlers) localFollowed(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	account := h.connected(personID)
	if account == nil || kept == nil || account.DID == kept.DID {
		return nil
	}
	return h.begin(ctx, account, repositories.SentFollow, kept.NodeID, kept.DID, 0)
}

// localUnfollowed is somebody here having stopped following somebody on
// Bluesky.
func (h *Handlers) localUnfollowed(ctx context.Context, personID int64, kept *repositories.RemoteActor) error {
	if !h.canWrite() || kept == nil {
		return nil
	}
	return h.takeBack(ctx, repositories.SentFollow, kept.NodeID, personID)
}

// localLiked is somebody here having liked something.
func (h *Handlers) localLiked(ctx context.Context, personID, nodeID int64) error {
	account := h.connected(personID)
	if account == nil {
		return nil
	}
	liked, err := h.Nodes.Read(nodeID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(liked)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentLike, liked.ID, subject, 0)
}

// localUnliked is somebody here having taken a like back.
func (h *Handlers) localUnliked(ctx context.Context, personID, nodeID int64) error {
	if !h.canWrite() {
		return nil
	}
	return h.takeBack(ctx, repositories.SentLike, nodeID, personID)
}

// localReposted is somebody here having reposted something.
func (h *Handlers) localReposted(ctx context.Context, repostID int64) error {
	if !h.canWrite() {
		return nil
	}
	repost, err := h.Nodes.Read(repostID)
	if err != nil || repost == nil || repost.ID == 0 {
		return err
	}
	if repost.Type != constants.NodeBaseRepostType || !repost.Enabled || repost.IsRemote() {
		return nil
	}
	if repost.CreatedBy <= 0 || repost.OwnerID != repost.CreatedBy {
		return nil
	}
	account := h.connected(repost.CreatedBy)
	if account == nil {
		return nil
	}
	post, err := h.Nodes.Read(repost.ParentID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(post)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentRepost, repost.ID, subject, 0)
}

// localPosted is somebody here having written something. A reply under
// something that is on Bluesky goes there; so does a note of their own
// that everybody may read.
func (h *Handlers) localPosted(ctx context.Context, noteID int64) error {
	if !h.canWrite() {
		return nil
	}
	note, err := h.Nodes.Read(noteID)
	if err != nil || note == nil || note.ID == 0 {
		return err
	}
	isPhoto := note.Type == constants.NodeMediumPhotoEntityType
	// A video post is written once it has its video.
	isVideo := note.Type == videos.NodeType && videos.FromMeta(note.Meta) != nil
	if (note.Type != constants.NodeMediumNoteEntityType && !isPhoto && !isVideo) || note.IsRemote() || !note.Enabled || note.DeletedAt != nil || note.Hidden {
		return nil
	}
	if note.CreatedBy <= 0 {
		return nil
	}
	// It came from their own account on Bluesky: it is there already.
	if markedOwn(note.Meta) {
		return nil
	}
	account := h.connected(note.CreatedBy)
	if account == nil {
		return nil
	}

	// A note that answers nothing: theirs, on their own profile, open to
	// everybody.
	if note.ParentID <= 0 && note.RootID <= 0 {
		if note.OwnerID != note.CreatedBy || note.Access != constants.AnahitaAccessPublic {
			return nil
		}
		return h.begin(ctx, account, repositories.SentPost, note.ID, "", note.ID)
	}

	parent, err := h.Nodes.Read(note.ParentID)
	if err != nil {
		return err
	}
	subject := h.onBluesky(parent)
	if subject == "" {
		return nil
	}
	return h.begin(ctx, account, repositories.SentPost, note.ID, subject, note.ID)
}

// localRemoved is something here being gone: a reply or a repost that
// went to Bluesky is removed there. The node is gone by now; what it was
// is in what is remembered of what was written.
func (h *Handlers) localRemoved(ctx context.Context, nodeID int64) error {
	if !h.canWrite() {
		return nil
	}
	for _, kind := range []string{repositories.SentPost, repositories.SentRepost} {
		if err := h.takeBack(ctx, kind, nodeID, 0); err != nil {
			return err
		}
	}
	return nil
}

// Write does one thing from the queue. What it answers decides whether
// the queue tries again.
func (h *Handlers) Write(ctx context.Context, delivery federation.Delivery) error {
	w := &write{}
	if err := json.Unmarshal(delivery.Payload, w); err != nil {
		return federation.Permanent(err)
	}
	account, err := h.Accounts.Read(delivery.ActorID)
	if err != nil {
		return err
	}
	if account == nil || account.BrokenAt != nil {
		return federation.Permanent(errors.New("no Bluesky account is connected"))
	}

	// Taken back while it waited: a like given and taken back before it
	// could be written is not written afterwards. What waits is not
	// always done in the order it was asked for, since what fails is
	// tried again later.
	if w.Op == "create" {
		if kind := kindOf(w.Collection); kind != "" {
			if still, err := h.Sent.IsOurs(account.DID, kind, w.RKey); err != nil || !still {
				return err
			}
		}
	}

	do := func(token string) error {
		if w.Op == "delete" {
			return h.Writer.DeleteRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey)
		}
		// A follow the account has already is not made a second time,
		// and is not this installation's to take back.
		if w.Collection == bsky.FollowCollection {
			already, err := h.Writer.Follows(ctx, account.PDS, token, w.Subject)
			if err != nil {
				return err
			}
			if already {
				return h.forgetSent(account, repositories.SentFollow, w.RKey)
			}
			return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey,
				&bsky.OutFollow{Type: bsky.FollowCollection, Subject: w.Subject, CreatedAt: bsky.Stamp(h.now())})
		}
		// What is to be written, made now: a post's words as they are
		// now, what it answers as Bluesky names it now, and its pictures
		// given to the account's server first.
		record, err := h.record(ctx, w, func(content []byte, mimeType string) (*bsky.OutBlob, error) {
			return h.Writer.UploadBlob(ctx, account.PDS, token, content, mimeType)
		})
		if err != nil || record == nil {
			return err
		}
		return h.Writer.CreateRecord(ctx, account.PDS, token, account.DID, w.Collection, w.RKey, record)
	}

	token, err := h.token(ctx, account, false)
	if err != nil {
		return err
	}
	err = do(token)
	if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
		// The short token ran out: another is got with the long one.
		if token, err = h.token(ctx, account, true); err != nil {
			return err
		}
		err = do(token)
	}
	if err != nil {
		log.Printf("writing to Bluesky for %d (%s %s): %v", account.PersonID, w.Op, w.Collection, err)
	}
	return err
}

// forgetSent forgets something that turned out not to need writing.
func (h *Handlers) forgetSent(account *repositories.Account, kind, rkey string) error {
	return h.Sent.DeleteByName(account.DID, kind, rkey)
}

// kindOf is what kind of thing a collection holds, as it is remembered.
func kindOf(collection string) string {
	for kind, each := range collections {
		if each == collection {
			return kind
		}
	}
	return ""
}

// record makes what is written for a like, a repost or a reply. Nil, and
// no error, when there is no longer anything to write.
func (h *Handlers) record(ctx context.Context, w *write, upload uploader) (any, error) {
	var (
		subject, root *bsky.Ref
		err           error
	)
	if w.Subject != "" {
		subject, root, err = h.Writer.RefOf(ctx, w.Subject)
		if errors.Is(err, bsky.ErrNobody) {
			// What it was of is gone from Bluesky.
			return nil, federation.Permanent(err)
		}
		if err != nil {
			return nil, err
		}
	} else if w.Collection != bsky.PostCollection {
		return nil, federation.Permanent(errors.New("a like or a repost of nothing"))
	}

	if w.Collection != bsky.PostCollection {
		return &bsky.OutSubject{Type: w.Collection, Subject: *subject, CreatedAt: bsky.Stamp(h.now())}, nil
	}

	note, err := h.Nodes.Read(w.NodeID)
	if err != nil {
		return nil, err
	}
	if note == nil || note.ID == 0 || note.DeletedAt != nil || !note.Enabled {
		// Deleted before it was written.
		return nil, nil
	}
	words := note.Body
	// A video post's title comes first, as its own line: a post on
	// Bluesky has no title of its own.
	if note.Type == videos.NodeType {
		if title := strings.TrimSpace(note.Name); title != "" {
			words = strings.TrimSpace(title + "\n\n" + words)
		}
	}
	post := bsky.NewOutPost(words, note.Language, note.CreatedAt)
	if subject != nil {
		post.Reply = &bsky.OutReply{Root: *root, Parent: *subject}
	}
	// Its pictures, when it is a photo post.
	if note.Type == constants.NodeMediumPhotoEntityType {
		if err := h.withPictures(post, note, upload); err != nil {
			return nil, err
		}
	}
	// Its video, when it is a video post.
	if note.Type == videos.NodeType {
		if err := h.withVideo(ctx, post, note, upload); err != nil {
			return nil, err
		}
	}
	if post.Text == "" && post.Embed == nil {
		return nil, nil
	}
	// The card of the link in it, as it was kept when the note was made.
	// A post has pictures or a card, not both.
	if card := linkpreviews.FromMeta(note.Meta); card != nil && post.Embed == nil {
		post.WithCard(card.URL, card.Title, card.Description, h.cardPicture(card.ImageKey, upload))
	}
	return post, nil
}

// withVideo gives a post the video of a video post: a size no taller than
// 720 and no heavier than Bluesky is sent, read from this installation's
// storage and given to the account's server; Bluesky makes it ready to
// play once the post is written. A video too long or too heavy to send,
// or one Bluesky would not take, goes as a card that leads to it, with
// its poster.
func (h *Handlers) withVideo(ctx context.Context, post *bsky.OutPost, note *entities.AnahitaNode, upload uploader) error {
	video := videos.FromMeta(note.Meta)
	if video == nil {
		return nil
	}
	if file := video.Shared(bsky.VideoMax); file != nil && video.Duration <= bsky.VideoSecondsMax && h.Transfers != nil {
		content, err := h.fetched(ctx, file.Key, bsky.VideoMax)
		if err != nil {
			// Storage may be back in a moment: tried again.
			return err
		}
		blob, err := upload(content, videos.MimeVideo)
		if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
			return err
		}
		if err == nil {
			post.WithVideo(blob, file.Width, file.Height)
			return nil
		}
		log.Printf("the video of post %d was not taken by Bluesky, and goes as a card: %v", note.ID, err)
	}

	title := strings.TrimSpace(note.Name)
	if title == "" {
		title = h.Site.Web(fmt.Sprintf("/videos/%d", note.ID))
	}
	var poster *bsky.OutBlob
	if video.Poster != "" && h.Keeper != nil && h.Keeper.Files != nil {
		if content, mimeType := h.stored(video.Poster); content != nil {
			poster, _ = upload(content, mimeType)
		}
	}
	post.WithCard(h.Site.Web(fmt.Sprintf("/videos/%d", note.ID)), title, "", poster)
	return nil
}

// fetched reads a file from this installation's storage, whole, at an
// address only the services reach. One heavier than max is refused.
func (h *Handlers) fetched(ctx context.Context, key string, max int64) ([]byte, error) {
	address, err := h.Transfers.FetchURL(key)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := h.Fetch
	if client == nil {
		client = &http.Client{Timeout: bsky.VideoTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading %s answered %d", key, res.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(res.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > max {
		return nil, federation.Permanent(fmt.Errorf("%s is heavier than %d bytes", key, max))
	}
	return content, nil
}

// uploader gives a file to the account's server.
type uploader func(content []byte, mimeType string) (*bsky.OutBlob, error)

// errPicturesNotThere is a photo post whose pictures are not listed yet:
// it is said to be made a moment before they are, and is tried again.
var errPicturesNotThere = errors.New("the post's pictures are not there yet")

// withPictures gives a post the pictures of a photo post: read from this
// installation's storage, made small enough for Bluesky to take, and
// given to the account's server.
func (h *Handlers) withPictures(post *bsky.OutPost, note *entities.AnahitaNode, upload uploader) error {
	if h.Keeper == nil || h.Keeper.PhotoFiles == nil || h.Keeper.Files == nil {
		return nil
	}

	var files []*entities.PhotoFile
	for waited := time.Duration(0); ; waited += time.Second {
		var err error
		if files, err = h.Keeper.PhotoFiles.BrowsePhotoFiles([]int64{note.ID}); err != nil {
			return err
		}
		if len(files) > 0 {
			break
		}
		if waited >= h.PicturesWait {
			return errPicturesNotThere
		}
		time.Sleep(time.Second)
	}

	for _, file := range files {
		if len(post.Embed.GetImages()) >= bsky.ImagesMax {
			break
		}
		sizes, err := note.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		content, mimeType := h.stored(sizes[entities.PortraitSizeOriginal].Url)
		if content == nil {
			continue
		}
		blob, err := upload(content, mimeType)
		if errors.Is(err, bsky.ErrExpired) || errors.Is(err, bsky.ErrDenied) {
			return err
		}
		if err != nil {
			log.Printf("a picture of post %d was not taken by Bluesky: %v", note.ID, err)
			continue
		}
		post.AddImage(blob, file.AltText, file.Width, file.Height)
	}
	return nil
}

// cardPicture is the picture of a link's card, given to the account's
// server, or nil: a card goes without one before a post goes late.
func (h *Handlers) cardPicture(key string, upload uploader) *bsky.OutBlob {
	if key == "" || h.Keeper == nil || h.Keeper.Files == nil {
		return nil
	}
	content, mimeType := h.stored(linkpreviews.StoragePrefix + key)
	if content == nil {
		return nil
	}
	blob, err := upload(content, mimeType)
	if err != nil {
		return nil
	}
	return blob
}

// stored reads a picture from this installation's storage, made smaller
// until it is one Bluesky takes. Nil when it is not there, or cannot be
// made small enough.
func (h *Handlers) stored(path string) ([]byte, string) {
	if path == "" {
		return nil, ""
	}
	read := services.NewFileRequest().Read
	read.Path = path
	file, err := h.Keeper.Files.Read(read)
	if err != nil || file == nil || len(file.Content) == 0 {
		return nil, ""
	}

	content := file.Content
	for _, width := range []int{0, 2048, 1600, 1280, 1024, 800, 640} {
		if width > 0 {
			if content, err = images.FitImage(file.Content, width); err != nil {
				return nil, ""
			}
		}
		if len(content) <= bsky.BlobMax {
			return content, file.Mimetype
		}
		// Still too heavy in the kind of file it is: a drawing or a
		// picture of a screen, kept without loss. As a photograph is
		// kept, it is a fraction of the weight.
		if lighter := asJPEG(content); lighter != nil && len(lighter) <= bsky.BlobMax {
			return lighter, "image/jpeg"
		}
	}
	return nil, ""
}

// asJPEG writes a picture again as a JPEG, on white where it could be
// seen through. Nil when it cannot be read.
func asJPEG(content []byte) []byte {
	decoded, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	flat := image.NewRGBA(decoded.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), decoded, decoded.Bounds().Min, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, flat, &jpeg.Options{Quality: 85}); err != nil {
		return nil
	}
	return out.Bytes()
}

// The short tokens, one for each connected person, kept only in memory.
type tokens struct {
	mu   sync.Mutex
	held map[int64]string
}

func (h *Handlers) remember(personID int64, token string) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	if h.tokens.held == nil {
		h.tokens.held = map[int64]string{}
	}
	h.tokens.held[personID] = token
}

func (h *Handlers) forgetToken(personID int64) {
	h.tokens.mu.Lock()
	defer h.tokens.mu.Unlock()
	delete(h.tokens.held, personID)
}

// token is the short token to write as an account with: the one held, or
// a new one got with the long token when there is none or `fresh` is
// asked for. An account whose long token is refused is marked as needing
// to be connected again.
func (h *Handlers) token(ctx context.Context, account *repositories.Account, fresh bool) (string, error) {
	h.tokens.mu.Lock()
	held := h.tokens.held[account.PersonID]
	h.tokens.mu.Unlock()
	if held != "" && !fresh {
		return held, nil
	}

	// The long token as it is kept now: another copy of this service may
	// have spent the one this copy read.
	current, err := h.Accounts.Read(account.PersonID)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", federation.Permanent(errors.New("no Bluesky account is connected"))
	}
	refresh, err := h.Crypt.Decrypt(current.Refresh)
	if err != nil {
		return "", federation.Permanent(err)
	}

	session, err := h.Writer.RefreshSession(ctx, current.PDS, refresh)
	if errors.Is(err, bsky.ErrDenied) || errors.Is(err, bsky.ErrExpired) {
		log.Printf("the Bluesky account of %d has to be connected again: %v", account.PersonID, err)
		if err := h.Accounts.SetBroken(account.PersonID); err != nil {
			log.Println("marking it so:", err)
		}
		h.forgetToken(account.PersonID)
		return "", federation.Permanent(err)
	}
	if err != nil {
		return "", err
	}

	kept, err := h.Crypt.Encrypt(session.RefreshJWT)
	if err != nil {
		return "", err
	}
	if err := h.Accounts.SetRefresh(account.PersonID, kept); err != nil {
		return "", err
	}
	h.remember(account.PersonID, session.AccessJWT)
	return session.AccessJWT, nil
}
