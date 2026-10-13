package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/federation"
	"anahita-libs/linkpreviews"
	"anahita-libs/site"
)

const (
	aliceDID = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	bobDID   = "did:plc:bbbbbbbbbbbbbbbbbbbbbbbb"
	aCID     = "bafkreihwihm6kpd6zuwhhlro75p5qks5qtrcu55jp3gddbfjsieiv7wuka"
)

// fakeNodes keeps what the graph would: people and posts from elsewhere,
// each by its address.
type fakeNodes struct {
	services.GraphNodeGRPCInterface
	kept   []*entities.AnahitaNode
	nextID int64
}

func (f *fakeNodes) find(match func(*entities.AnahitaNode) bool) *entities.AnahitaNode {
	for _, node := range f.kept {
		if match(node) {
			return node
		}
	}
	return nil
}

func copied(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	if node == nil {
		return nil, nil
	}
	c := *node
	return &c, nil
}

func (f *fakeNodes) Read(id int64) (*entities.AnahitaNode, error) {
	return copied(f.find(func(n *entities.AnahitaNode) bool { return n.ID == id }))
}

func (f *fakeNodes) ReadByURI(uri string) (*entities.AnahitaNode, error) {
	return copied(f.find(func(n *entities.AnahitaNode) bool { return n.URI == uri }))
}

func (f *fakeNodes) UpsertRemote(node *entities.AnahitaNode) (*entities.AnahitaNode, bool, error) {
	// A handle is one person's.
	if node.Alias != "" && f.find(func(n *entities.AnahitaNode) bool { return n.Alias == node.Alias && n.URI != node.URI }) != nil {
		return nil, false, fmt.Errorf("%s is somebody already known", node.Alias)
	}
	if known := f.find(func(n *entities.AnahitaNode) bool { return n.URI == node.URI }); known != nil {
		id, kind, avatar, cover := known.ID, known.Type, known.AvatarFilename, known.CoverFilename
		*known = *node
		known.ID, known.Type = id, kind
		if known.AvatarFilename == "" {
			known.AvatarFilename = avatar
		}
		if known.CoverFilename == "" {
			known.CoverFilename = cover
		}
		saved, _ := copied(known)
		return saved, false, nil
	}
	f.nextID++
	added := *node
	added.ID = 100 + f.nextID
	f.kept = append(f.kept, &added)
	saved, _ := copied(&added)
	return saved, true, nil
}

func (f *fakeNodes) DeleteRemote(uri string, ownerID int64) (bool, error) {
	for i, node := range f.kept {
		if node.URI == uri && node.OwnerID == ownerID {
			f.kept = append(f.kept[:i], f.kept[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeNodes) DeleteRemoteActor(uri string) (bool, error) {
	actor := f.find(func(n *entities.AnahitaNode) bool { return n.URI == uri })
	if actor == nil {
		return false, nil
	}
	var left []*entities.AnahitaNode
	for _, node := range f.kept {
		if node.ID != actor.ID && node.OwnerID != actor.ID {
			left = append(left, node)
		}
	}
	f.kept = left
	return true, nil
}

// fakeActors is what this service keeps of people on Bluesky, and who is
// followed from here.
type fakeActors struct {
	kept     map[string]*repositories.RemoteActor
	followed map[int64]bool
	undone   []int64
}

func (f *fakeActors) Save(actor *repositories.RemoteActor) error {
	if known := f.kept[actor.DID]; known != nil {
		actor.BackfilledAt = known.BackfilledAt
	}
	saved := *actor
	f.kept[actor.DID] = &saved
	return nil
}

func (f *fakeActors) ReadByDID(did string) (*repositories.RemoteActor, error) {
	if known := f.kept[did]; known != nil {
		c := *known
		return &c, nil
	}
	return nil, nil
}

func (f *fakeActors) ReadByNode(id int64) (*repositories.RemoteActor, error) {
	for _, known := range f.kept {
		if known.NodeID == id {
			c := *known
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeActors) Delete(id int64) error {
	for did, known := range f.kept {
		if known.NodeID == id {
			delete(f.kept, did)
		}
	}
	return nil
}

func (f *fakeActors) IsFollowedFromHere(id int64) (bool, error) { return f.followed[id], nil }

func (f *fakeActors) Followed(limit int) ([]string, error) {
	var dids []string
	for did, known := range f.kept {
		if f.followed[known.NodeID] {
			dids = append(dids, did)
		}
	}
	return dids, nil
}

func (f *fakeActors) NotBackfilled(limit int) ([]*repositories.RemoteActor, error) {
	var waiting []*repositories.RemoteActor
	for _, known := range f.kept {
		if f.followed[known.NodeID] && known.BackfilledAt == nil {
			c := *known
			waiting = append(waiting, &c)
		}
	}
	return waiting, nil
}

func (f *fakeActors) BeginBackfill(id int64) (bool, error) {
	for _, known := range f.kept {
		if known.NodeID == id && known.BackfilledAt == nil {
			now := time.Now()
			known.BackfilledAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeActors) UndoBackfill(id int64) error {
	f.undone = append(f.undone, id)
	for _, known := range f.kept {
		if known.NodeID == id {
			known.BackfilledAt = nil
		}
	}
	return nil
}

// fakeBluesky answers as the network's reader would.
type fakeBluesky struct {
	profiles map[string]*bsky.Profile
	feeds    map[string][]bsky.FeedItem
	posts    map[string]*bsky.Post
	// Who an account follows, a page at a time: "did|" then "did|next".
	following map[string][]bsky.Profile
	asked     []string
	err       error
}

func (f *fakeBluesky) Profile(_ context.Context, actor string) (*bsky.Profile, error) {
	f.asked = append(f.asked, actor)
	if f.err != nil {
		return nil, f.err
	}
	if profile := f.profiles[actor]; profile != nil {
		c := *profile
		return &c, nil
	}
	return nil, bsky.ErrNobody
}

func (f *fakeBluesky) Following(_ context.Context, did, cursor string) ([]bsky.Profile, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	page := f.following[did+"|"+cursor]
	next := ""
	if _, more := f.following[did+"|next"]; more && cursor == "" {
		next = "next"
	}
	return page, next, nil
}

func (f *fakeBluesky) AuthorFeed(_ context.Context, did string, limit int) ([]bsky.FeedItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.feeds[did], nil
}

func (f *fakeBluesky) PostAt(_ context.Context, uri string) (*bsky.Post, error) {
	if post := f.posts[uri]; post != nil {
		return post, nil
	}
	return nil, bsky.ErrNobody
}

// fakeReposts keeps who reposted what, as the graph does, and which post
// each repost heard was of.
type fakeReposts struct {
	services.RepostGRPCInterface
	made  map[string]bool
	names map[string]int64
}

func (f *fakeReposts) IsReposted(actorID, nodeID int64) (bool, error) {
	return f.made[fmt.Sprint(actorID, ">", nodeID)], nil
}

func (f *fakeReposts) Add(req *services.RepostAddRequest) (*entities.AnahitaNode, error) {
	f.made[fmt.Sprint(req.OwnerID, ">", req.ParentID)] = true
	return &entities.AnahitaNode{ID: 9000 + req.ParentID}, nil
}

func (f *fakeReposts) Delete(req *services.RepostDeleteRequest) error {
	delete(f.made, fmt.Sprint(req.AuthorID, ">", req.ParentID))
	return nil
}

func (f *fakeReposts) Save(actorID int64, rkey string, parentID int64) error {
	f.names[fmt.Sprint(actorID, "/", rkey)] = parentID
	return nil
}

func (f *fakeReposts) Take(actorID int64, rkey string) (int64, error) {
	key := fmt.Sprint(actorID, "/", rkey)
	parentID := f.names[key]
	delete(f.names, key)
	return parentID, nil
}

type fakeBlocks map[string]bool

func (f fakeBlocks) IsBlocked(domain string) bool {
	for {
		if f[domain] {
			return true
		}
		dot := strings.Index(domain, ".")
		if dot < 0 {
			return false
		}
		domain = domain[dot+1:]
	}
}

type fakePictures struct {
	asked   []string
	content []byte
}

func (f *fakePictures) Bytes(_ context.Context, address string, _ int64) ([]byte, string, error) {
	f.asked = append(f.asked, address)
	return f.content, "image/png", nil
}

type fakeFiles struct {
	services.FilesGRPCInterface
	stored  map[string]int
	deleted []string
	// What can be read back, by where it is.
	content map[string][]byte
}

func (f *fakeFiles) Read(req *services.FileReadRequest) (*entities.File, error) {
	content, ok := f.content[req.Path]
	if !ok {
		return nil, errors.New("no such file")
	}
	return &entities.File{Content: content, Path: req.Path, Mimetype: "image/png", Size: int64(len(content))}, nil
}

func (f *fakeFiles) Add(req *services.FileAddRequest) (*entities.File, error) {
	f.stored[req.Path] = len(req.Content)
	return &entities.File{}, nil
}

func (f *fakeFiles) Delete(req *services.FileDeleteRequest) error {
	delete(f.stored, req.Path)
	f.deleted = append(f.deleted, req.Path)
	return nil
}

type fakePhotoFiles struct {
	kept map[int64][]*entities.PhotoFile
}

func (f *fakePhotoFiles) BrowsePhotoFiles(ids []int64) ([]*entities.PhotoFile, error) {
	var all []*entities.PhotoFile
	for _, id := range ids {
		all = append(all, f.kept[id]...)
	}
	return all, nil
}

func (f *fakePhotoFiles) SetPhotoFiles(id int64, files []*entities.PhotoFile, _ []int64) ([]*entities.PhotoFile, error) {
	f.kept[id] = files
	return files, nil
}

// fakeCards finds a card for any address, and remembers which were asked
// for.
type fakeCards struct {
	asked []string
}

func (f *fakeCards) Preview(_ context.Context, address string) *linkpreviews.Preview {
	f.asked = append(f.asked, address)
	return nil
}

func aPicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for x := 0; x < 600; x++ {
		for y := 0; y < 400; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// aHeavyPicture is a picture that weighs more than Bluesky takes: noise,
// which does not press small.
func aHeavyPicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1500, 1000))
	seed := uint32(1)
	for i := range img.Pix {
		seed = seed*1664525 + 1013904223
		img.Pix[i] = uint8(seed >> 24)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	if out.Len() <= bsky.BlobMax {
		t.Fatalf("the heavy picture weighs only %d", out.Len())
	}
	return out.Bytes()
}

// world is an installation that reads Bluesky, with everything it talks
// to stood in for.
type world struct {
	t        *testing.T
	h        *Handlers
	nodes    *fakeNodes
	actors   *fakeActors
	bluesky  *fakeBluesky
	pictures *fakePictures
	files    *fakeFiles
	listed   *fakePhotoFiles
	cards    *fakeCards
	reposts  *fakeReposts
	said     []string
	changed  int
}

func newWorld(t *testing.T) *world {
	w := &world{
		t:        t,
		nodes:    &fakeNodes{},
		actors:   &fakeActors{kept: map[string]*repositories.RemoteActor{}, followed: map[int64]bool{}},
		pictures: &fakePictures{content: aPicture(t)},
		files:    &fakeFiles{stored: map[string]int{}, content: map[string][]byte{}},
		listed:   &fakePhotoFiles{kept: map[int64][]*entities.PhotoFile{}},
		cards:    &fakeCards{},
		reposts:  &fakeReposts{made: map[string]bool{}, names: map[string]int64{}},
		bluesky: &fakeBluesky{
			feeds: map[string][]bsky.FeedItem{},
			posts: map[string]*bsky.Post{},
			profiles: map[string]*bsky.Profile{
				"alice.bsky.social": {
					DID: aliceDID, Handle: "alice.bsky.social", DisplayName: "Alice",
					Description: "Beans & <b>peas</b>\nsecond line",
					Avatar:      "https://cdn.bsky.app/img/avatar/plain/" + aliceDID + "/" + aCID,
					Banner:      "https://cdn.bsky.app/img/banner/plain/" + aliceDID + "/" + aCID,
				},
			},
		},
	}
	w.bluesky.profiles[aliceDID] = w.bluesky.profiles["alice.bsky.social"]

	w.h = NewHandlers(&Env{
		Site:    site.Addresses{Domain: "anahita.example", WebURL: "https://anahita.example", APIURL: "https://anahita.example"},
		Nodes:   w.nodes,
		Actors:  w.actors,
		Bluesky: w.bluesky,
		Blocks:  fakeBlocks{},
		Reposts: w.reposts, RepostRecords: w.reposts,
		Keeper:  &federation.Keeper{Nodes: w.nodes, Files: w.files, PhotoFiles: w.listed, Pictures: w.pictures},
		Cards:   w.cards,
		CDN:     "https://cdn.bsky.app",
		Now:     func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC) },
		Changed: func() { w.changed++ },
		Announce: func(payload *event.EventPayload, name string) error {
			w.said = append(w.said, fmt.Sprintf("%s %d by %d", name, payload.ObjectID, payload.ViewerID))
			return nil
		},
	})
	return w
}

// alice is Alice looked up, and followed by somebody here when `followed`.
func (w *world) alice(followed bool) *repositories.RemoteActor {
	w.t.Helper()
	person, code := w.h.lookup(context.Background(), "alice.bsky.social")
	if person == nil {
		w.t.Fatalf("alice was not found: %s", code)
	}
	w.actors.followed[person.ID] = followed
	kept, _ := w.actors.ReadByDID(aliceDID)
	return kept
}

func (w *world) keptAt(uri string) *entities.AnahitaNode {
	node, _ := w.nodes.ReadByURI(uri)
	return node
}

func aPost(t *testing.T, raw string) *bsky.Post {
	t.Helper()
	post := &bsky.Post{}
	if err := json.Unmarshal([]byte(raw), post); err != nil {
		t.Fatal(err)
	}
	return post
}

func TestSomebodyOnBlueskyIsKeptAsAPersonOfAnotherServer(t *testing.T) {
	w := newWorld(t)

	person, code := w.h.lookup(context.Background(), " @Alice.Bsky.Social ")
	if person == nil {
		t.Fatal(code)
	}
	if person.Type != constants.NodeActorATProtoPersonEntityType || person.Alias != "alice.bsky.social" || person.Name != "Alice" ||
		person.URI != "at://"+aliceDID || person.OriginDomain != "bsky.app" || person.URL != "https://bsky.app/profile/"+aliceDID || person.Locked {
		t.Errorf("kept as %+v", person)
	}
	// What they say of themselves is words, whatever is written in it.
	if strings.Contains(person.Body, "<b>") || !strings.Contains(person.Body, "peas") || !strings.Contains(person.Body, "second line") {
		t.Errorf("body %q", person.Body)
	}

	// Their pictures are copied here, from where Bluesky keeps them.
	node := w.keptAt("at://" + aliceDID)
	// Asked for as a kind of picture this installation reads.
	if got := strings.Join(w.pictures.asked, " "); got != "https://cdn.bsky.app/img/avatar/plain/"+aliceDID+"/"+aCID+"@jpeg https://cdn.bsky.app/img/banner/plain/"+aliceDID+"/"+aCID+"@jpeg" {
		t.Errorf("asked for %q", got)
	}
	if node.AvatarFilename == "" || node.CoverFilename == "" || len(w.pictures.asked) != 2 || len(w.files.stored) == 0 {
		t.Errorf("avatar %q, cover %q, asked %v", node.AvatarFilename, node.CoverFilename, w.pictures.asked)
	}

	// Found again, they are the same one, and pictures that have not
	// changed are not fetched again.
	again, _ := w.h.lookup(context.Background(), "https://bsky.app/profile/"+aliceDID)
	if again == nil || again.ID != person.ID || len(w.pictures.asked) != 2 {
		t.Errorf("found again as %+v, pictures asked %d times", again, len(w.pictures.asked))
	}
}

func TestWhoIsNotLookedUp(t *testing.T) {
	w := newWorld(t)
	w.h.Blocks = fakeBlocks{"refused.example": true}
	w.bluesky.profiles[bobDID] = &bsky.Profile{DID: bobDID, Handle: "bob.refused.example"}

	for query, want := range map[string]string{
		"alice":                                 errLookupQuery,
		"alice@mastodon.example":                errLookupQuery,
		"anahita.example":                       errLookupOurs,
		"ana.anahita.example":                   errLookupOurs,
		"bob.refused.example":                   errLookupBlocked,
		bobDID:                                  errLookupBlocked,
		"nobody.bsky.social":                    errLookupNobody,
		"https://example.com/x.y":               errLookupQuery,
		"did:plc:" + "cccccccccccccccccccccccc": errLookupNobody,
	} {
		if person, code := w.h.lookup(context.Background(), query); person != nil || code != want {
			t.Errorf("%q: %+v, %s; want %s", query, person, code, want)
		}
	}
	if len(w.nodes.kept) != 0 {
		t.Errorf("%d kept", len(w.nodes.kept))
	}

	// Bluesky as a whole refused: nobody on it is asked about.
	w.h.Blocks = fakeBlocks{"bsky.app": true}
	asked := len(w.bluesky.asked)
	if person, code := w.h.lookup(context.Background(), "alice.bsky.social"); person != nil || code != errLookupBlocked || len(w.bluesky.asked) != asked {
		t.Errorf("with Bluesky refused: %+v, %s", person, code)
	}

	// The reader being away is not nobody being there.
	w.h.Blocks = fakeBlocks{}
	w.bluesky.err = errors.New("timeout")
	if _, code := w.h.lookup(context.Background(), "alice.bsky.social"); code != errLookupFailed {
		t.Errorf("with the reader away: %s", code)
	}
}

// A handle can pass from one account to another. Whoever has it now is
// called by it only once whoever had it is no longer kept by it.
func TestAHandleSomebodyElseIsKeptBy(t *testing.T) {
	w := newWorld(t)
	w.alice(false)

	w.bluesky.profiles["alice.bsky.social"] = &bsky.Profile{DID: bobDID, Handle: "alice.bsky.social"}
	person, code := w.h.lookup(context.Background(), "alice.bsky.social")
	if person == nil || person.Alias != "did-plc-bbbbbbbbbbbbbbbbbbbbbbbb.invalid" || person.URI != "at://"+bobDID {
		t.Fatalf("%+v, %s", person, code)
	}

	// And one whose handle leads nowhere is called the same way.
	if got := aliasOf(&bsky.Profile{DID: aliceDID, Handle: "handle.invalid"}); got != "did-plc-aaaaaaaaaaaaaaaaaaaaaaaa.invalid" {
		t.Errorf("called %q", got)
	}
}

// THE RULE THIS GUARDS: a post from Bluesky is kept only when somebody
// here follows whoever wrote it, and never a reply.
func TestAPostIsKeptOnlyOfSomebodyFollowed(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(false)
	uri := bsky.PostURI(aliceDID, "3l3qo2vutsw2b")

	post := aPost(t, `{"text":"The beans are up #garden","createdAt":"2026-10-11T08:30:00Z","langs":["en"]}`)
	if err := w.h.keepPost(alice, "3l3qo2vutsw2b", post); err != nil || w.keptAt(uri) != nil {
		t.Fatalf("kept of somebody nobody follows: %v", err)
	}

	w.actors.followed[alice.NodeID] = true
	if err := w.h.keepPost(alice, "3l3qo2vutsw2b", post); err != nil {
		t.Fatal(err)
	}
	kept := w.keptAt(uri)
	if kept == nil || kept.Type != constants.NodeMediumNoteEntityType || kept.Body != "The beans are up #garden" ||
		kept.OwnerID != alice.NodeID || kept.OriginDomain != "bsky.app" || kept.Language != "en" ||
		!kept.CreatedAt.Equal(time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("kept as %+v", kept)
	}

	// Heard again, it is the same post, and said to be changed, not new.
	if err := w.h.keepPost(alice, "3l3qo2vutsw2b", post); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%s %d by %d|%s %d by %d", event.RemoteNoteEvent(true), kept.ID, alice.NodeID, event.RemoteNoteEvent(false), kept.ID, alice.NodeID)
	if got := strings.Join(w.said, "|"); got != want {
		t.Errorf("said %q\nwant %q", got, want)
	}

	reply := aPost(t, `{"text":"Mine too","reply":{"root":{"uri":"at://x"},"parent":{"uri":"at://x"}}}`)
	if err := w.h.keepPost(alice, "3l3reply", reply); err != nil || w.keptAt(bsky.PostURI(aliceDID, "3l3reply")) != nil {
		t.Errorf("a reply was kept: %v", err)
	}
	if err := w.h.keepPost(alice, "3l3empty", aPost(t, `{"text":" "}`)); err != nil || w.keptAt(bsky.PostURI(aliceDID, "3l3empty")) != nil {
		t.Errorf("a post with nothing in it was kept: %v", err)
	}
}

// THE RULE THIS GUARDS: a post that comes with pictures is a photo post
// here, with its pictures copied into this installation's own storage.
func TestAPostWithPicturesIsAPhotoPost(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)
	w.pictures.asked = nil

	const pictures = `"embed":{"$type":"app.bsky.embed.images","images":[
		{"alt":"A row of beans","image":{"ref":{"$link":"` + aCID + `"},"mimeType":"image/jpeg"}}]}`

	if err := w.h.keepPost(alice, "3l3photo", aPost(t, `{"text":"The beans",`+pictures+`}`)); err != nil {
		t.Fatal(err)
	}
	kept := w.keptAt(bsky.PostURI(aliceDID, "3l3photo"))
	if kept == nil || kept.Type != constants.NodeMediumPhotoEntityType || kept.Body != "The beans" {
		t.Fatalf("kept as %+v", kept)
	}
	if got := strings.Join(w.pictures.asked, " "); got != "https://cdn.bsky.app/img/feed_fullsize/plain/"+aliceDID+"/"+aCID+"@jpeg" {
		t.Errorf("asked for %q", got)
	}
	if its := w.listed.kept[kept.ID]; len(its) != 1 || its[0].AltText != "A row of beans" || its[0].Filename == "" {
		t.Fatalf("listed %+v", its)
	}

	// With no words it is still a post: its pictures are what it says.
	if err := w.h.keepPost(alice, "3l3only", aPost(t, `{"text":"",`+pictures+`}`)); err != nil || w.keptAt(bsky.PostURI(aliceDID, "3l3only")) == nil {
		t.Errorf("a post of pictures alone was not kept: %v", err)
	}

	// What its author marked as not to be shown plainly is kept for its
	// words, without its pictures.
	w.pictures.asked = nil
	marked := `{"text":"Warned of",` + pictures + `,"labels":{"values":[{"val":"graphic-media"}]}}`
	if err := w.h.keepPost(alice, "3l3marked", aPost(t, marked)); err != nil {
		t.Fatal(err)
	}
	if kept := w.keptAt(bsky.PostURI(aliceDID, "3l3marked")); kept == nil || kept.Type != constants.NodeMediumNoteEntityType || len(w.pictures.asked) != 0 {
		t.Errorf("kept as %+v, pictures asked %v", kept, w.pictures.asked)
	}

	// Taken away there, it goes from here with what was stored of it.
	stored := len(w.files.stored)
	if err := w.h.removePost(alice, "3l3photo"); err != nil {
		t.Fatal(err)
	}
	if w.keptAt(bsky.PostURI(aliceDID, "3l3photo")) != nil || len(w.files.stored) >= stored || !strings.HasPrefix(w.said[len(w.said)-1], event.RemoteNoteDeleted) {
		t.Errorf("after it was taken away: %d files of %d, said %q", len(w.files.stored), stored, w.said[len(w.said)-1])
	}
	// Somebody else cannot take it away.
	w.bluesky.profiles["bob.bsky.social"] = &bsky.Profile{DID: bobDID, Handle: "bob.bsky.social"}
	w.h.lookup(context.Background(), "bob.bsky.social")
	bob, _ := w.actors.ReadByDID(bobDID)
	bob.DID = aliceDID
	if err := w.h.removePost(bob, "3l3only"); err != nil || w.keptAt(bsky.PostURI(aliceDID, "3l3only")) == nil {
		t.Errorf("a post was taken away by somebody whose it is not: %v", err)
	}
}

func TestTheCardAndTheQuoteOfAPost(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)

	// The page a post carries a card of is the one asked about, though
	// its words do not name it.
	carried := `{"text":"Worth reading","embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com/story","title":"T","description":"D"}}}`
	if err := w.h.keepPost(alice, "3l3card", aPost(t, carried)); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(w.cards.asked, " "); got != "https://example.com/story" {
		t.Errorf("card asked of %q", got)
	}

	// A post that quotes another leads to it.
	w.cards.asked = nil
	quoting := `{"text":"This","embed":{"$type":"app.bsky.embed.record","record":{"uri":"at://` + bobDID + `/app.bsky.feed.post/3l3quoted"}}}`
	if err := w.h.keepPost(alice, "3l3quote", aPost(t, quoting)); err != nil {
		t.Fatal(err)
	}
	const quotedPage = "https://bsky.app/profile/" + bobDID + "/post/3l3quoted"
	if kept := w.keptAt(bsky.PostURI(aliceDID, "3l3quote")); kept == nil || kept.Body != "This\n\n↪ [bsky.app]("+quotedPage+")" {
		t.Errorf("kept as %+v", kept)
	}
	if got := strings.Join(w.cards.asked, " "); got != quotedPage {
		t.Errorf("card asked of %q", got)
	}
}

func TestWhatIsHeard(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)
	ctx := context.Background()

	heard := func(raw string) {
		t.Helper()
		e := &bsky.Event{}
		if err := json.Unmarshal([]byte(raw), e); err != nil {
			t.Fatal(err)
		}
		w.h.Heard(ctx, e)
	}
	uri := bsky.PostURI(aliceDID, "3l3heard")

	heard(`{"did":"` + aliceDID + `","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"3l3heard","record":{"text":"Heard","createdAt":"2026-10-12T11:00:00Z"}}}`)
	if kept := w.keptAt(uri); kept == nil || kept.Body != "Heard" {
		t.Fatalf("kept as %+v", kept)
	}

	// Of somebody not kept here, nothing is kept.
	heard(`{"did":"` + bobDID + `","time_us":2,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"3l3bob","record":{"text":"Bob"}}}`)
	if w.keptAt(bsky.PostURI(bobDID, "3l3bob")) != nil {
		t.Error("a post of somebody not kept here was kept")
	}
	// A like, or anything else that is not a post, is nothing here.
	heard(`{"did":"` + aliceDID + `","time_us":3,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.like","rkey":"3l3like","record":{"text":"not a post"}}}`)
	if len(w.nodes.kept) != 2 {
		t.Errorf("%d kept", len(w.nodes.kept))
	}

	heard(`{"did":"` + aliceDID + `","time_us":4,"kind":"commit","commit":{"operation":"delete","collection":"app.bsky.feed.post","rkey":"3l3heard"}}`)
	if w.keptAt(uri) != nil {
		t.Error("a post taken away is still kept")
	}

	// They changed what they say of themselves, and the handle they go by.
	w.bluesky.profiles[aliceDID] = &bsky.Profile{DID: aliceDID, Handle: "alice.example.org", DisplayName: "Alice B"}
	heard(`{"did":"` + aliceDID + `","time_us":5,"kind":"commit","commit":{"operation":"update","collection":"app.bsky.actor.profile","rkey":"self","record":{}}}`)
	if node := w.keptAt("at://" + aliceDID); node.Name != "Alice B" || node.Alias != "alice.example.org" || node.ID != alice.NodeID {
		t.Errorf("after a change: %+v", node)
	}

	// Put away for now, they are left as they are. Closed, they go, with
	// what they posted, and are no longer listened for.
	heard(`{"did":"` + aliceDID + `","time_us":6,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"3l3last","record":{"text":"Last"}}}`)
	heard(`{"did":"` + aliceDID + `","time_us":7,"kind":"account","account":{"active":false,"status":"deactivated"}}`)
	if w.keptAt("at://"+aliceDID) == nil {
		t.Fatal("somebody put away for now was removed")
	}
	changed := w.changed
	heard(`{"did":"` + aliceDID + `","time_us":8,"kind":"account","account":{"active":false,"status":"deleted"}}`)
	if len(w.nodes.kept) != 0 || len(w.actors.kept) != 0 || w.changed != changed+1 {
		t.Errorf("after the account was closed: %d nodes, %d actors", len(w.nodes.kept), len(w.actors.kept))
	}
}

func feedItem(did, rkey, text string, reason string) bsky.FeedItem {
	item := bsky.FeedItem{}
	item.Post.URI = bsky.PostURI(did, rkey)
	item.Post.Author.DID = did
	item.Post.Record = json.RawMessage(`{"text":"` + text + `","createdAt":"2026-10-10T10:00:00Z"}`)
	if reason != "" {
		item.Reason = json.RawMessage(reason)
	}
	return item
}

func TestWhatWasPostedBeforeTheyWereFollowed(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)

	// Newest first, as the reader gives it: with something of Bob's that
	// Alice reposted, and a reply the reader let through.
	reply := feedItem(aliceDID, "3l3reply", "A reply", "")
	reply.Post.Record = json.RawMessage(`{"text":"A reply","reply":{"root":{"uri":"at://x"}}}`)
	w.bluesky.feeds[aliceDID] = []bsky.FeedItem{
		feedItem(aliceDID, "3l3c", "Third", ""),
		feedItem(bobDID, "3l3bob", "Bob's", `{"$type":"app.bsky.feed.defs#reasonRepost"}`),
		reply,
		feedItem(aliceDID, "3l3b", "Second", ""),
		feedItem(aliceDID, "3l3a", "First", ""),
	}

	w.h.backfill(alice)

	var bodies []string
	for _, node := range w.nodes.kept[1:] {
		bodies = append(bodies, node.Body)
	}
	// Kept oldest first, and only what is theirs.
	if got := strings.Join(bodies, ","); got != "First,Second,Third" {
		t.Errorf("kept %q", got)
	}

	// Once: asked again, nothing is read.
	w.bluesky.feeds[aliceDID] = append(w.bluesky.feeds[aliceDID], feedItem(aliceDID, "3l3z", "Later", ""))
	w.h.backfill(alice)
	w.h.CatchUp()
	if len(w.nodes.kept) != 4 {
		t.Errorf("%d kept after a second reading", len(w.nodes.kept))
	}
}

// A reading that fails is tried again: by what looks, now and then, for
// who is followed and has not had their earlier posts read.
func TestAReadingThatFailsIsTriedAgain(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)
	w.bluesky.feeds[aliceDID] = []bsky.FeedItem{feedItem(aliceDID, "3l3a", "First", "")}

	w.bluesky.err = errors.New("timeout")
	w.h.backfill(alice)
	if len(w.nodes.kept) != 1 || len(w.actors.undone) != 1 {
		t.Fatalf("%d kept, claim given back %d times", len(w.nodes.kept), len(w.actors.undone))
	}

	w.bluesky.err = nil
	w.h.CatchUp()
	if len(w.nodes.kept) != 2 || w.changed == 0 {
		t.Errorf("%d kept after catching up", len(w.nodes.kept))
	}
}

func TestSomebodyHereFollowsSomebodyOnBluesky(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(false)
	w.bluesky.feeds[aliceDID] = []bsky.FeedItem{feedItem(aliceDID, "3l3a", "First", "")}

	// Somebody here, followed by somebody here: nothing of this service's.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{ID: 7, Type: constants.NodeActorPersonEntityType, Alias: "ana"})
	if err := w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: 8, TargetID: 7}); err != nil || w.changed != 0 {
		t.Fatalf("a follow of somebody here: %v, changed %d", err, w.changed)
	}

	// The graph has the follow by the time it is said.
	w.actors.followed[alice.NodeID] = true
	if err := w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: 7, TargetID: alice.NodeID}); err != nil {
		t.Fatal(err)
	}
	if w.keptAt(bsky.PostURI(aliceDID, "3l3a")) == nil || w.changed != 1 {
		t.Errorf("after the follow: changed %d", w.changed)
	}
	if wanted, _ := w.h.Wanted(); len(wanted) != 1 || wanted[0] != aliceDID {
		t.Errorf("wanted %v", wanted)
	}

	w.actors.followed[alice.NodeID] = false
	if err := w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.deleted.v1", ObjectID: 7, TargetID: alice.NodeID}); err != nil || w.changed != 2 {
		t.Errorf("after the unfollow: %v, changed %d", err, w.changed)
	}
	if wanted, _ := w.h.Wanted(); len(wanted) != 0 {
		t.Errorf("wanted %v", wanted)
	}
}

// THE RULE THIS GUARDS: a reply is kept only under something that is kept
// here, by somebody followed from here.
func TestAReplyIsKeptUnderWhatItAnswers(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)

	if err := w.h.keepPost(alice, "3l3first", aPost(t, `{"text":"A long thing, begun"}`)); err != nil {
		t.Fatal(err)
	}
	first := w.keptAt(bsky.PostURI(aliceDID, "3l3first"))

	answer := `{"text":"and gone on with","reply":{"root":{"uri":"` + first.URI + `"},"parent":{"uri":"` + first.URI + `"}}}`
	if err := w.h.keepPost(alice, "3l3second", aPost(t, answer)); err != nil {
		t.Fatal(err)
	}
	second := w.keptAt(bsky.PostURI(aliceDID, "3l3second"))
	if second == nil || second.ParentID != first.ID || second.OwnerID != alice.NodeID {
		t.Fatalf("kept as %+v", second)
	}

	// Under something that is not kept here, it is not kept.
	stray := `{"text":"to somebody else","reply":{"parent":{"uri":"at://` + bobDID + `/app.bsky.feed.post/3l3bob"}}}`
	if err := w.h.keepPost(alice, "3l3stray", aPost(t, stray)); err != nil || w.keptAt(bsky.PostURI(aliceDID, "3l3stray")) != nil {
		t.Errorf("a reply to something not kept here was kept: %v", err)
	}
	// Nor by somebody nobody follows, under something that is.
	w.bluesky.profiles["bob.bsky.social"] = &bsky.Profile{DID: bobDID, Handle: "bob.bsky.social"}
	w.h.lookup(context.Background(), "bob.bsky.social")
	bob, _ := w.actors.ReadByDID(bobDID)
	if err := w.h.keepPost(bob, "3l3bobs", aPost(t, answer)); err != nil || w.keptAt(bsky.PostURI(bobDID, "3l3bobs")) != nil {
		t.Errorf("a reply by somebody nobody follows was kept: %v", err)
	}
}

// THE RULE THIS GUARDS: what somebody followed reposts is kept with
// whoever wrote it, though nobody here follows them, and only for that.
func TestWhatSomebodyFollowedReposts(t *testing.T) {
	w := newWorld(t)
	alice := w.alice(true)
	ctx := context.Background()

	bobs := bsky.PostURI(bobDID, "3l3bob")
	w.bluesky.profiles[bobDID] = &bsky.Profile{DID: bobDID, Handle: "bob.bsky.social", DisplayName: "Bob"}
	w.bluesky.posts[bobs] = aPost(t, `{"text":"Bob said this"}`)

	heard := func(raw string) {
		t.Helper()
		e := &bsky.Event{}
		if err := json.Unmarshal([]byte(raw), e); err != nil {
			t.Fatal(err)
		}
		w.h.Heard(ctx, e)
	}
	repost := `{"did":"` + aliceDID + `","time_us":1,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.repost","rkey":"3l3rp","record":{"subject":{"uri":"` + bobs + `","cid":"c"}}}}`

	heard(repost)
	post := w.keptAt(bobs)
	bob := w.keptAt("at://" + bobDID)
	if post == nil || bob == nil || post.OwnerID != bob.ID || post.Body != "Bob said this" || bob.Alias != "bob.bsky.social" {
		t.Fatalf("kept %+v by %+v", post, bob)
	}
	if !w.reposts.made[fmt.Sprint(alice.NodeID, ">", post.ID)] {
		t.Fatal("no repost was made")
	}

	// Heard twice, it is one repost. A post of Bob's own is still not
	// kept: nobody here follows him.
	heard(repost)
	if len(w.reposts.made) != 1 {
		t.Errorf("%d reposts", len(w.reposts.made))
	}
	heard(`{"did":"` + bobDID + `","time_us":2,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.post","rkey":"3l3more","record":{"text":"More from Bob"}}}`)
	if w.keptAt(bsky.PostURI(bobDID, "3l3more")) != nil {
		t.Error("a post of somebody nobody follows was kept")
	}

	// Taken back, which says only the repost's name.
	heard(`{"did":"` + aliceDID + `","time_us":3,"kind":"commit","commit":{"operation":"delete","collection":"app.bsky.feed.repost","rkey":"3l3rp"}}`)
	if len(w.reposts.made) != 0 || len(w.reposts.names) != 0 {
		t.Errorf("after it was taken back: %v", w.reposts.made)
	}

	// A reply, or something that is gone, is not fetched to be reposted.
	w.bluesky.posts[bsky.PostURI(bobDID, "3l3reply")] = aPost(t, `{"text":"x","reply":{"parent":{"uri":"`+bobs+`"}}}`)
	for _, rkey := range []string{"3l3reply", "3l3gone"} {
		heard(`{"did":"` + aliceDID + `","time_us":4,"kind":"commit","commit":{"operation":"create","collection":"app.bsky.feed.repost","rkey":"r` + rkey + `","record":{"subject":{"uri":"` + bsky.PostURI(bobDID, rkey) + `"}}}}`)
	}
	if len(w.reposts.made) != 0 {
		t.Errorf("reposted what is not kept: %v", w.reposts.made)
	}
}
