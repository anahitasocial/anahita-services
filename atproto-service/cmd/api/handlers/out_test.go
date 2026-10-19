package handlers

import (
	"anahita-libs/policy"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
)

const (
	anaID  = 7
	anaDID = "did:plc:cccccccccccccccccccccccc"
	anaPDS = "https://pds.example"
)

type fakeAccounts struct {
	kept map[int64]*repositories.Account
}

func (f *fakeAccounts) Read(id int64) (*repositories.Account, error) {
	if known := f.kept[id]; known != nil {
		c := *known
		return &c, nil
	}
	return nil, nil
}

func (f *fakeAccounts) WhoConnected(did string) (int64, error) {
	for id, known := range f.kept {
		if known.DID == did {
			return id, nil
		}
	}
	return 0, nil
}

func (f *fakeAccounts) Shadowed() ([]string, error) { return nil, nil }

func (f *fakeAccounts) Working() ([]*repositories.Account, error) {
	var working []*repositories.Account
	for _, known := range f.kept {
		if known.BrokenAt == nil {
			c := *known
			working = append(working, &c)
		}
	}
	return working, nil
}

func (f *fakeAccounts) SetNotified(id int64, at time.Time) error {
	f.kept[id].NotifiedAt = at
	return nil
}

// fakeGraph is likes, follows and who has blocked whom, as the graph
// keeps them. Each of the three is asked through a stand-in of its own,
// since two of them add by a method of the same name.
type fakeGraph struct {
	likes   map[string]bool
	follows map[string]bool
	blocked map[string]bool
}

type fakeLikes struct {
	services.LikesGRPCInterface
	*fakeGraph
}

func (f fakeLikes) IsLiked(source, target int64) (bool, error) {
	return f.likes[fmt.Sprint(source, ">", target)], nil
}

func (f fakeLikes) Add(source, target *entities.AnahitaNode, _ *entities.Viewer) error {
	f.likes[fmt.Sprint(source.ID, ">", target.ID)] = true
	return nil
}

func (f fakeLikes) Delete(source, target *entities.AnahitaNode) error {
	delete(f.likes, fmt.Sprint(source.ID, ">", target.ID))
	return nil
}

type fakeFollows struct {
	services.SocialgraphFollowGRPCInterface
	*fakeGraph
}

func (f fakeFollows) Add(req *services.SocialgraphFollowAddRequest) error {
	f.follows[fmt.Sprint(req.Source.ID, ">", req.Target.ID)] = true
	return nil
}

func (f fakeFollows) Delete(req *services.SocialgraphFollowDeleteRequest) error {
	delete(f.follows, fmt.Sprint(req.Source.ID, ">", req.Target.ID))
	return nil
}

type fakeRelations struct {
	services.SocialgraphGRPCInterface
	*fakeGraph
}

func (f fakeRelations) Read(a, b int64) (*entities.ActorRelation, error) {
	return &entities.ActorRelation{
		IsBlocking:  f.blocked[fmt.Sprint(a, ">", b)],
		IsLeading:   f.follows[fmt.Sprint(b, ">", a)],
		IsFollowing: f.follows[fmt.Sprint(a, ">", b)],
	}, nil
}

func (f *fakeAccounts) Save(account *repositories.Account) error {
	for id, known := range f.kept {
		if known.DID == account.DID && id != account.PersonID {
			return repositories.ErrAccountTaken
		}
	}
	c := *account
	f.kept[account.PersonID] = &c
	return nil
}

func (f *fakeAccounts) SetRefresh(id int64, refresh string) error {
	f.kept[id].Refresh = refresh
	return nil
}

func (f *fakeAccounts) SetBroken(id int64) error {
	if f.kept[id].BrokenAt == nil {
		at := time.Now()
		f.kept[id].BrokenAt = &at
	}
	return nil
}

func (f *fakeAccounts) Delete(id int64) error {
	delete(f.kept, id)
	return nil
}

type fakeSent struct {
	rows []*repositories.Sent
	next int64
}

func (f *fakeSent) Begin(sent *repositories.Sent) (bool, error) {
	for _, row := range f.rows {
		if row.Kind == sent.Kind && row.PersonID == sent.PersonID && row.LocalID == sent.LocalID {
			return false, nil
		}
	}
	f.next++
	c := *sent
	c.ID = f.next
	f.rows = append(f.rows, &c)
	return true, nil
}

func (f *fakeSent) Read(kind string, localID, personID int64) (*repositories.Sent, error) {
	for _, row := range f.rows {
		if row.Kind == kind && row.LocalID == localID && (personID == 0 || row.PersonID == personID) {
			c := *row
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeSent) Delete(id int64) error {
	var left []*repositories.Sent
	for _, row := range f.rows {
		if row.ID != id {
			left = append(left, row)
		}
	}
	f.rows = left
	return nil
}

func (f *fakeSent) DeleteAllOf(personID int64) error {
	var left []*repositories.Sent
	for _, row := range f.rows {
		if row.PersonID != personID {
			left = append(left, row)
		}
	}
	f.rows = left
	return nil
}

func (f *fakeSent) ReadByName(did, kind, rkey string) (*repositories.Sent, error) {
	for _, row := range f.rows {
		if row.DID == did && row.Kind == kind && row.RKey == rkey {
			c := *row
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeSent) DeleteByName(did, kind, rkey string) error {
	var left []*repositories.Sent
	for _, row := range f.rows {
		if !(row.DID == did && row.Kind == kind && row.RKey == rkey) {
			left = append(left, row)
		}
	}
	f.rows = left
	return nil
}

func (f *fakeSent) IsOurs(did, kind, rkey string) (bool, error) {
	for _, row := range f.rows {
		if row.DID == did && row.Kind == kind && row.RKey == rkey {
			return true, nil
		}
	}
	return false, nil
}

// fakeWriter is an account's server, and what Bluesky names posts by.
type fakeWriter struct {
	password string
	// The short tokens it takes, and the long one that gets the next.
	access   string
	refresh  string
	written  []string
	records  map[string]any
	uploaded []int
	// Who the account follows already, on Bluesky.
	following map[string]bool
	// What the account was told of, newest first.
	told []bsky.Notification
}

func (f *fakeWriter) PDSOf(_ context.Context, _, did string) (string, error) { return anaPDS, nil }

func (f *fakeWriter) CreateSession(_ context.Context, _, identifier, password string) (*bsky.Session, error) {
	if password != f.password {
		return nil, bsky.ErrDenied
	}
	return &bsky.Session{DID: identifier, AccessJWT: f.access, RefreshJWT: f.refresh}, nil
}

func (f *fakeWriter) RefreshSession(_ context.Context, _, refresh string) (*bsky.Session, error) {
	if refresh != f.refresh {
		return nil, bsky.ErrDenied
	}
	f.access, f.refresh = f.access+"+", f.refresh+"+"
	return &bsky.Session{AccessJWT: f.access, RefreshJWT: f.refresh}, nil
}

func (f *fakeWriter) CreateRecord(_ context.Context, pds, token, did, collection, rkey string, record any) error {
	if token != f.access {
		return bsky.ErrExpired
	}
	f.written = append(f.written, "create "+collection)
	f.records[collection+"/"+rkey] = record
	return nil
}

func (f *fakeWriter) DeleteRecord(_ context.Context, pds, token, did, collection, rkey string) error {
	if token != f.access {
		return bsky.ErrExpired
	}
	f.written = append(f.written, "delete "+collection)
	delete(f.records, collection+"/"+rkey)
	return nil
}

func (f *fakeWriter) UploadBlob(_ context.Context, pds, token string, content []byte, mimeType string) (*bsky.OutBlob, error) {
	if token != f.access {
		return nil, bsky.ErrExpired
	}
	f.uploaded = append(f.uploaded, len(content))
	blob := &bsky.OutBlob{Type: "blob", MimeType: mimeType, Size: int64(len(content))}
	blob.Ref.Link = fmt.Sprintf("blob%d", len(f.uploaded))
	return blob, nil
}

func (f *fakeWriter) Notifications(_ context.Context, pds, token string, limit int) ([]bsky.Notification, error) {
	if token != f.access {
		return nil, bsky.ErrExpired
	}
	return f.told, nil
}

func (f *fakeWriter) Follows(_ context.Context, pds, token, did string) (bool, error) {
	if token != f.access {
		return false, bsky.ErrExpired
	}
	return f.following[did], nil
}

func (f *fakeWriter) RefOf(_ context.Context, uri string) (*bsky.Ref, *bsky.Ref, error) {
	if strings.Contains(uri, "gone") {
		return nil, nil, bsky.ErrNobody
	}
	return &bsky.Ref{URI: uri, CID: "cid-of-" + uri[len(uri)-6:]}, &bsky.Ref{URI: "at://root", CID: "cid-root"}, nil
}

// GetRecord reads a record back as the server gives it: as JSON.
func (f *fakeWriter) GetRecord(_ context.Context, pds, did, collection, rkey string) (map[string]any, error) {
	record, ok := f.records[collection+"/"+rkey]
	if !ok {
		return nil, bsky.ErrNobody
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	return value, json.Unmarshal(raw, &value)
}

// fakeQueue is the table of what waits. As in the table, a key is kept
// while what it names waits, and is free again once that was done.
type fakeQueue struct {
	waiting []federation.Delivery
	keys    map[string]bool
	keyOf   map[string]string
}

func (f *fakeQueue) Enqueue(_ context.Context, actorID int64, target string, payload []byte, key string) error {
	if f.keys[key] {
		return nil
	}
	f.keys[key] = true
	if f.keyOf == nil {
		f.keyOf = map[string]string{}
	}
	f.keyOf[string(payload)] = key
	f.waiting = append(f.waiting, federation.Delivery{ActorID: actorID, Target: target, Payload: payload})
	return nil
}

// A stand-in that marks what it has kept, so that a token kept without
// being put through it is seen.
type fakeCrypt struct{}

func (fakeCrypt) Encrypt(plain string) (string, error) { return "enc(" + plain + ")", nil }
func (fakeCrypt) Decrypt(kept string) (string, error) {
	if !strings.HasPrefix(kept, "enc(") {
		return "", errors.New("not something that was encrypted")
	}
	return strings.TrimSuffix(strings.TrimPrefix(kept, "enc("), ")"), nil
}

type writing struct {
	*world
	accounts *fakeAccounts
	sent     *fakeSent
	writer   *fakeWriter
	queue    *fakeQueue
}

// writingWorld is a world where Ana, somebody here, may connect a Bluesky
// account, and Alice on Bluesky is followed and has posted.
func writingWorld(t *testing.T) (*writing, *entities.AnahitaNode) {
	w := &writing{
		world:    newWorld(t),
		accounts: &fakeAccounts{kept: map[int64]*repositories.Account{}},
		sent:     &fakeSent{},
		writer:   &fakeWriter{password: "abcd-efgh", access: "access", refresh: "refresh", records: map[string]any{}},
		queue:    &fakeQueue{keys: map[string]bool{}},
	}
	w.h.Accounts, w.h.Sent, w.h.Writer, w.h.Queue, w.h.Crypt = w.accounts, w.sent, w.writer, w.queue, fakeCrypt{}
	w.h.Directory = "https://plc.example"

	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{ID: anaID, Type: constants.NodeActorPersonEntityType, Alias: "ana", Enabled: true})
	w.bluesky.profiles["ana.example.org"] = &bsky.Profile{DID: anaDID, Handle: "ana.example.org"}

	alice := w.alice(true)
	if err := w.h.keepPost(alice, "3l3post", aPost(t, `{"text":"The beans are up"}`)); err != nil {
		t.Fatal(err)
	}
	return w, w.keptAt(bsky.PostURI(aliceDID, "3l3post"))
}

func (w *writing) connect() {
	w.t.Helper()
	if account, code := w.h.connect(context.Background(), anaID, "ana.example.org", "abcd-efgh"); account == nil {
		w.t.Fatal(code)
	}
}

// drain writes everything that waits, and says what could not be.
func (w *writing) drain() []error {
	var failed []error
	for len(w.queue.waiting) > 0 {
		next := w.queue.waiting[0]
		w.queue.waiting = w.queue.waiting[1:]
		delete(w.queue.keys, w.queue.keyOf[string(next.Payload)])
		if err := w.h.Write(context.Background(), next); err != nil {
			failed = append(failed, err)
		}
	}
	return failed
}

func (w *writing) heard(name string, payload event.EventPayload) {
	w.t.Helper()
	payload.Name = name
	if err := w.h.HandleEvent(&payload); err != nil {
		w.t.Fatal(err)
	}
}

// THE RULE THIS GUARDS: the password an account is connected with is not
// kept, and the token it is exchanged for is kept only encrypted.
func TestConnectingABlueskyAccount(t *testing.T) {
	w, _ := writingWorld(t)
	ctx := context.Background()

	if account, code := w.h.connect(ctx, anaID, "ana.example.org", "wrong"); account != nil || code != errConnectDenied {
		t.Fatalf("with a wrong password: %+v, %s", account, code)
	}
	if account, code := w.h.connect(ctx, anaID, "nobody.example.org", "abcd-efgh"); account != nil || code != errConnectNobody {
		t.Fatalf("an account that is not there: %+v, %s", account, code)
	}

	w.connect()
	kept := w.accounts.kept[anaID]
	if kept.DID != anaDID || kept.Handle != "ana.example.org" || kept.PDS != anaPDS || kept.Refresh != "enc(refresh)" {
		t.Fatalf("kept %+v", kept)
	}
	if raw, _ := json.Marshal(kept); strings.Contains(string(raw), "abcd-efgh") {
		t.Error("the password was kept")
	}

	// One account is one person's here.
	if account, code := w.h.connect(ctx, 8, "ana.example.org", "abcd-efgh"); account != nil || code != errConnectTaken {
		t.Errorf("connected by a second person: %+v, %s", account, code)
	}
}

func TestALikeARepostAndAReplyGoToBluesky(t *testing.T) {
	w, post := writingWorld(t)

	// Not connected: what Ana does here stays here.
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if len(w.queue.waiting) != 0 || len(w.sent.rows) != 0 {
		t.Fatal("something was written for somebody who connected nothing")
	}

	w.connect()

	// A like. Said twice on the bus, it is written once.
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})

	// A repost.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 50, Type: constants.NodeBaseRepostType, Enabled: true, OwnerID: anaID, CreatedBy: anaID, ParentID: post.ID,
	})
	w.heard("node.repost.repost.note.added.v1", event.EventPayload{ObjectID: 50})

	// A reply, and Ana answering her own reply.
	reply := &entities.AnahitaNode{
		ID: 60, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		ParentID: post.ID, RootID: post.ID, Body: "Mine too, see [this](https://example.com/beans) #garden", Language: "en",
	}
	second := &entities.AnahitaNode{
		ID: 61, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		ParentID: 60, RootID: post.ID, Body: "And more",
	}
	w.nodes.kept = append(w.nodes.kept, reply, second)
	note := event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType}
	note.ObjectID = 60
	w.heard("node.medium.text-service.note.added.v1", note)
	note.ObjectID = 61
	w.heard("node.medium.text-service.note.added.v1", note)
	// Changed afterwards: Bluesky has no changing a post, and it is not
	// written again.
	w.heard("node.medium.text-service.note.edited.v1", note)

	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if got := strings.Join(w.writer.written, ", "); got != "create app.bsky.feed.like, create app.bsky.feed.repost, create app.bsky.feed.post, create app.bsky.feed.post" {
		t.Fatalf("written: %s", got)
	}

	replied := w.sent.rows[2]
	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+replied.RKey])
	for _, want := range []string{
		`"text":"Mine too, see this #garden"`, `"uri":"https://example.com/beans"`, `"tag":"garden"`, `"langs":["en"]`,
		`"parent":{"uri":"` + post.URI + `"`, `"root":{"uri":"at://root"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the reply as written lacks %s:\n%s", want, out)
		}
	}
	// The second answers the first, by the name the first was written
	// under.
	under, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[3].RKey])
	if want := `"parent":{"uri":"` + bsky.PostURI(anaDID, replied.RKey) + `"`; !strings.Contains(string(under), want) {
		t.Errorf("the second reply lacks %s:\n%s", want, under)
	}

	// THE RULE THIS GUARDS: when Bluesky tells of what was written from
	// here, it is not kept a second time as something from elsewhere.
	if !w.h.ours(anaDID, repositories.SentPost, replied.RKey) || w.h.ours(anaDID, repositories.SentPost, "3l3other") || w.h.ours(aliceDID, repositories.SentPost, replied.RKey) {
		t.Error("what was written from here is not known for it")
	}

	// Taken back: the like, the repost and the first reply.
	w.heard("edge.voteup.note.deleted.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	w.heard("node.repost.repost.note.deleted.v1", event.EventPayload{ObjectID: 50})
	note.ObjectID = 60
	w.heard("node.medium.text-service.note.deleted.v1", note)
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if len(w.writer.records) != 1 || len(w.sent.rows) != 1 {
		t.Errorf("after taking back: %d records there, %d remembered", len(w.writer.records), len(w.sent.rows))
	}
}

func TestWhatIsNotWritten(t *testing.T) {
	w, post := writingWorld(t)
	w.connect()

	// Something that is not on Bluesky: a note written here.
	w.nodes.kept = append(w.nodes.kept,
		&entities.AnahitaNode{ID: 70, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: 9, CreatedBy: 9, Body: "Local"},
		&entities.AnahitaNode{ID: 71, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID, ParentID: 70, Body: "Under a local note"},
		// Notes of Ana's that answer nothing: one for her followers only,
		// and one on somebody else's profile.
		&entities.AnahitaNode{ID: 72, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID, Access: "followers", Body: "For followers"},
		&entities.AnahitaNode{ID: 73, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: 9, CreatedBy: anaID, Access: constants.AnahitaAccessPublic, Body: "On another profile"},
	)
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: 70})
	note := event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType}
	for _, id := range []int64{71, 72, 73} {
		note.ObjectID = id
		w.heard("node.medium.text-service.note.added.v1", note)
	}
	// And a like by somebody who connected nothing.
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: 9, ObjectID: post.ID})
	if len(w.queue.waiting) != 0 {
		t.Fatalf("%d things wait to be written", len(w.queue.waiting))
	}
}

func TestTokensThatRunOut(t *testing.T) {
	w, post := writingWorld(t)
	w.connect()

	// The short token has run out at the server: the long one gets the
	// next, and what it is replaced by is kept, encrypted.
	w.writer.access = "another"
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if len(w.writer.written) != 1 || w.accounts.kept[anaID].Refresh != "enc(refresh+)" {
		t.Fatalf("written %v, kept %s", w.writer.written, w.accounts.kept[anaID].Refresh)
	}

	// The long token is refused too: the account has to be connected
	// again, and what waited is given up, not tried for two days.
	w.writer.access, w.writer.refresh = "new", "revoked"
	w.heard("edge.voteup.note.deleted.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	failed := w.drain()
	if len(failed) != 1 || !errors.Is(failed[0], federation.ErrPermanent) || w.accounts.kept[anaID].BrokenAt == nil {
		t.Fatalf("failed %v, broken %v", failed, w.accounts.kept[anaID].BrokenAt)
	}
	if w.h.connected(anaID) != nil {
		t.Error("an account that has to be connected again is still written as")
	}

	// Connected again, it works.
	w.writer.access, w.writer.refresh = "access", "refresh"
	w.connect()
	if w.h.connected(anaID) == nil {
		t.Error("connected again and still not written as")
	}
}

// THE RULE THIS GUARDS: one person is one profile. A Bluesky account that
// somebody here connected is not also kept as somebody of another server.
func TestAConnectedAccountIsNotKeptAsSomebodyElsewhere(t *testing.T) {
	w, _ := writingWorld(t)
	ctx := context.Background()

	// Before connecting, Ana looked her own account up, followed it, and
	// a post of hers was kept.
	w.bluesky.profiles[anaDID] = w.bluesky.profiles["ana.example.org"]
	shadow, _ := w.h.lookup(ctx, "ana.example.org")
	if shadow == nil || shadow.Type != constants.NodeActorATProtoPersonEntityType {
		t.Fatalf("%+v", shadow)
	}
	w.actors.followed[shadow.ID] = true
	kept, _ := w.actors.ReadByDID(anaDID)
	if err := w.h.keepPost(kept, "3l3mine", aPost(t, `{"text":"Posted on Bluesky"}`)); err != nil {
		t.Fatal(err)
	}

	w.connect()

	if w.keptAt("at://"+anaDID) != nil || w.keptAt(bsky.PostURI(anaDID, "3l3mine")) != nil || len(w.actors.kept) != 1 {
		t.Fatal("what was kept of the account as somebody elsewhere is still there")
	}

	// Looked up now, it is Ana who is found, and nothing is kept.
	found, code := w.h.lookup(ctx, "ana.example.org")
	if found == nil || found.ID != anaID || found.Type != constants.NodeActorPersonEntityType || found.Alias != "ana" {
		t.Fatalf("%+v, %s", found, code)
	}
	if w.keptAt("at://"+anaDID) != nil {
		t.Error("kept again by being looked up")
	}

	// Nor is it kept again for a post of hers that somebody followed
	// reposts.
	mine := bsky.PostURI(anaDID, "3l3mine")
	w.bluesky.posts[mine] = aPost(t, `{"text":"Posted on Bluesky"}`)
	w.h.Heard(ctx, &bsky.Event{DID: aliceDID, Kind: "commit", Commit: &struct {
		Operation  string          `json:"operation"`
		Collection string          `json:"collection"`
		RKey       string          `json:"rkey"`
		Record     json.RawMessage `json:"record"`
	}{"create", bsky.RepostCollection, "3l3rp", json.RawMessage(`{"subject":{"uri":"` + mine + `"}}`)}})
	if w.keptAt("at://"+anaDID) != nil || w.keptAt(mine) != nil || len(w.reposts.made) != 0 {
		t.Error("kept again by being reposted")
	}
}

// THE RULE THIS GUARDS: of what somebody writes that answers nothing, only
// a note everybody may read, on their own profile, is posted to Bluesky.
func TestANoteOfOnesOwnGoesToBluesky(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	card := linkpreviews.IntoMeta(nil, &linkpreviews.Snapshot{URL: "https://example.com/story", Title: "The story", Description: "What it says"})
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 80, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Anahita to Bluesky #HelloWorld https://example.com/story", Meta: card,
	})
	note := event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: 80}
	w.heard("node.medium.text-service.note.added.v1", note)
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}

	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	for _, want := range []string{
		`"text":"Anahita to Bluesky #HelloWorld https://example.com/story"`, `"tag":"HelloWorld"`,
		`"embed":{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com/story","title":"The story","description":"What it says"}}`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the note as written lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(string(out), `"reply"`) {
		t.Errorf("a note that answers nothing was written as a reply: %s", out)
	}

	// Deleted here, it goes from there.
	w.heard("node.medium.text-service.note.deleted.v1", note)
	if failed := w.drain(); len(failed) != 0 || len(w.writer.records) != 0 {
		t.Errorf("after it was deleted: %v, %d there", failed, len(w.writer.records))
	}
}

// THE RULE THIS GUARDS: what is taken back before it could be written is
// not written afterwards. What fails is tried again later, by when what
// took it back may have been done already.
func TestALikeTakenBackBeforeItWasWritten(t *testing.T) {
	w, post := writingWorld(t)
	w.connect()

	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	w.heard("edge.voteup.note.deleted.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})

	// The taking back is done first, as when the like failed once and
	// waits to be tried again.
	w.queue.waiting[0], w.queue.waiting[1] = w.queue.waiting[1], w.queue.waiting[0]
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if len(w.writer.records) != 0 {
		t.Errorf("a like that was taken back was written: %v", w.writer.written)
	}
}

// THE RULE THIS GUARDS: a photo post goes to Bluesky with its pictures,
// each small enough for Bluesky to take and with what it shows.
func TestAPhotoPostGoesWithItsPictures(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	photo := &entities.AnahitaNode{
		ID: 90, Type: constants.NodeMediumPhotoEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "The beans",
	}
	w.nodes.kept = append(w.nodes.kept, photo)
	post := event.EventPayload{ObjectType: constants.NodeMediumPhotoEntityType, ObjectID: 90}
	w.heard("node.medium.photo-service.photo.added.v1", post)

	// Said to be made before its pictures are listed: tried again, and
	// nothing is written meanwhile.
	waiting := w.queue.waiting[0]
	if err := w.h.Write(context.Background(), waiting); !errors.Is(err, errPicturesNotThere) || len(w.writer.records) != 0 {
		t.Fatalf("before its pictures were there: %v, %d written", err, len(w.writer.records))
	}

	// Its pictures: one as it is, and one too heavy for Bluesky, which is
	// made smaller.
	sizes, _ := photo.PortraitsFor("one.png")
	w.files.content[sizes[entities.PortraitSizeOriginal].Url] = aPicture(t)
	heavy, _ := photo.PortraitsFor("two.png")
	w.files.content[heavy[entities.PortraitSizeOriginal].Url] = aHeavyPicture(t)
	w.listed.kept[90] = []*entities.PhotoFile{
		{Filename: "one.png", Mimetype: "image/png", Width: 600, Height: 400, AltText: "A row of beans"},
		{Filename: "two.png", Mimetype: "image/png", Width: 3000, Height: 2000},
		{Filename: "gone.png", Mimetype: "image/png"},
	}
	if err := w.h.Write(context.Background(), waiting); err != nil {
		t.Fatal(err)
	}

	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	for _, want := range []string{
		`"text":"The beans"`, `"embed":{"$type":"app.bsky.embed.images","images":[{"alt":"A row of beans","image":{"$type":"blob","ref":{"$link":"blob1"}`,
		`"aspectRatio":{"width":600,"height":400}`, `"$link":"blob2"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the photo post as written lacks %s:\n%s", want, out)
		}
	}
	if len(w.writer.uploaded) != 2 || w.writer.uploaded[1] > bsky.BlobMax {
		t.Errorf("uploaded %v", w.writer.uploaded)
	}
}

// A photo post too long for Bluesky keeps its pictures, and its words end
// with a link to it here (roadmap row 100): a post has pictures or a card,
// not both.
func TestAPhotoPostCutLinksToTheRest(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	photo := &entities.AnahitaNode{
		ID: 91, Type: constants.NodeMediumPhotoEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: strings.Repeat("beans ", 80),
	}
	w.nodes.kept = append(w.nodes.kept, photo)
	sizes, _ := photo.PortraitsFor("one.png")
	w.files.content[sizes[entities.PortraitSizeOriginal].Url] = aPicture(t)
	w.listed.kept[91] = []*entities.PhotoFile{{Filename: "one.png", Mimetype: "image/png", Width: 600, Height: 400}}
	w.heard("node.medium.photo-service.photo.added.v1", event.EventPayload{ObjectType: constants.NodeMediumPhotoEntityType, ObjectID: 91})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}

	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	for _, want := range []string{`…\n\nanahita.example/photos/91"`, `"uri":"https://anahita.example/photos/91"`, `"app.bsky.embed.images"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the post as written lacks %s:\n%s", want, out)
		}
	}
}

// More pictures than Bluesky takes: the first four, and a link to the
// post here, where they all are (roadmap row 100).
func TestAPhotoPostWithMorePicturesThanBlueskyTakes(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	photo := &entities.AnahitaNode{
		ID: 92, Type: constants.NodeMediumPhotoEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "The garden",
	}
	w.nodes.kept = append(w.nodes.kept, photo)
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("%d.png", i)
		sizes, _ := photo.PortraitsFor(name)
		w.files.content[sizes[entities.PortraitSizeOriginal].Url] = aPicture(t)
		w.listed.kept[92] = append(w.listed.kept[92], &entities.PhotoFile{Filename: name, Mimetype: "image/png", Width: 600, Height: 400})
	}
	w.heard("node.medium.photo-service.photo.added.v1", event.EventPayload{ObjectType: constants.NodeMediumPhotoEntityType, ObjectID: 92})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}

	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	if want := `"text":"The garden\n\nanahita.example/photos/92"`; !strings.Contains(string(out), want) {
		t.Errorf("the post as written lacks %s:\n%s", want, out)
	}
	if len(w.writer.uploaded) != bsky.ImagesMax || strings.Count(string(out), `"image":`) != bsky.ImagesMax {
		t.Errorf("%d uploaded:\n%s", len(w.writer.uploaded), out)
	}
}

func TestTheCardOfALinkGoesWithItsPicture(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	w.files.content[linkpreviews.StoragePrefix+"card.jpg"] = aPicture(t)
	card := linkpreviews.IntoMeta(nil, &linkpreviews.Snapshot{URL: "https://example.com/story", Title: "The story", ImageKey: "card.jpg"})
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 95, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Read this https://example.com/story", Meta: card,
	})
	w.heard("node.medium.text-service.note.added.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: 95})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	if want := `"thumb":{"$type":"blob","ref":{"$link":"blob1"}`; !strings.Contains(string(out), want) {
		t.Errorf("the card lacks %s:\n%s", want, out)
	}
}

// An article goes to Bluesky as a post of its title and opening, cut to
// fit, with a card that leads to it here, its cover the card's picture. A
// topic stays here (the user, 2026-10-17).
func TestAnArticleGoesToBlueskyWithACardToIt(t *testing.T) {
	w, _ := writingWorld(t)
	w.connect()

	article := &entities.AnahitaNode{
		ID: 96, Type: constants.NodeMediumArticleEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Name: "Beans in October",
		Body: "The **beans** went in late this year. " + strings.Repeat("Rain fell and the wind blew. ", 20), CoverFilename: "cover.jpg",
	}
	covers, _ := article.GetCovers()
	w.files.content[covers[entities.CoverSizeMedium].Url] = aPicture(t)
	topic := &entities.AnahitaNode{
		ID: 97, Type: "node.medium.text-service.topic.v1", Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Name: "A topic", Body: "Talk about beans",
	}
	w.nodes.kept = append(w.nodes.kept, article, topic)

	w.heard("node.medium.text-service.topic.added.v1", event.EventPayload{ObjectType: topic.Type, ObjectID: 97})
	w.heard("node.medium.text-service.article.added.v1", event.EventPayload{ObjectType: article.Type, ObjectID: 96})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if len(w.sent.rows) != 1 || w.sent.rows[0].LocalID != 96 {
		t.Fatalf("written: %+v", w.sent.rows)
	}
	out, _ := json.Marshal(w.writer.records["app.bsky.feed.post/"+w.sent.rows[0].RKey])
	for _, want := range []string{
		`"text":"Beans in October\n\nThe beans went in late this year.`,
		`"$type":"app.bsky.embed.external"`, `/articles/96"`, `"title":"Beans in October"`,
		`"description":"The beans went in late this year. Rain fell`,
		`"thumb":{"$type":"blob"`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the article as written lacks %s:\n%s", want, out)
		}
	}
}

// THE RULE THIS GUARDS: a follow is written to Bluesky only when the
// account does not follow already, and only what was written from here is
// taken back from here.
func TestAFollowIsToldToBluesky(t *testing.T) {
	w, _ := writingWorld(t)
	alice, _ := w.actors.ReadByDID(aliceDID)
	w.bluesky.profiles["bob.bsky.social"] = &bsky.Profile{DID: bobDID, Handle: "bob.bsky.social"}
	w.h.lookup(context.Background(), "bob.bsky.social")
	bob, _ := w.actors.ReadByDID(bobDID)
	follow := func(name string, target int64) {
		w.heard(name, event.EventPayload{ObjectID: anaID, TargetID: target})
	}

	// Not connected: the follow is here and nowhere else.
	follow("edge.follow.socialgraph.added.v1", alice.NodeID)
	if len(w.queue.waiting) != 0 {
		t.Fatal("a follow was written for somebody who connected nothing")
	}

	w.connect()
	// Ana's account follows Bob already, from Bluesky itself.
	w.writer.following = map[string]bool{bobDID: true}

	follow("edge.follow.socialgraph.added.v1", alice.NodeID)
	follow("edge.follow.socialgraph.added.v1", bob.NodeID)
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if got := strings.Join(w.writer.written, ", "); got != "create app.bsky.graph.follow" {
		t.Fatalf("written: %s", got)
	}
	out, _ := json.Marshal(w.writer.records["app.bsky.graph.follow/"+w.sent.rows[0].RKey])
	if want := `"subject":"` + aliceDID + `"`; !strings.Contains(string(out), want) || len(w.sent.rows) != 1 {
		t.Fatalf("the follow as written: %s, %d remembered", out, len(w.sent.rows))
	}

	// Unfollowed here: Alice's goes from Bluesky, and Bob's, which was
	// made there, is left.
	follow("edge.follow.socialgraph.deleted.v1", alice.NodeID)
	follow("edge.follow.socialgraph.deleted.v1", bob.NodeID)
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if got := strings.Join(w.writer.written, ", "); got != "create app.bsky.graph.follow, delete app.bsky.graph.follow" || len(w.writer.records) != 0 {
		t.Errorf("written: %s", got)
	}
}

// THE RULE THIS GUARDS: what somebody on Bluesky does to a post that went
// out from here is kept here, whoever they are, and nothing they do to
// anything else is.
func TestWhatPeopleOnBlueskyDoToWhatWasWrittenFromHere(t *testing.T) {
	w, _ := writingWorld(t)
	graph := &fakeGraph{likes: map[string]bool{}, follows: map[string]bool{}, blocked: map[string]bool{}}
	w.h.Likes, w.h.FollowEdges, w.h.Relations = fakeLikes{fakeGraph: graph}, fakeFollows{fakeGraph: graph}, fakeRelations{fakeGraph: graph}
	w.connect()

	// A note of Ana's that went to Bluesky.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 80, Type: constants.NodeMediumNoteEntityType, Service: "text-service", Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Anahita to Bluesky",
	})
	w.heard("node.medium.text-service.note.added.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: 80})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	mine := bsky.PostURI(anaDID, w.sent.rows[0].RKey)
	w.said = nil

	// Bob, whom nobody here follows, likes it, reposts it, answers it and
	// follows Ana. He also likes something that never went out from here.
	told := func(reason, subject, record, at string) bsky.Notification {
		n := bsky.Notification{URI: "at://" + bobDID + "/app.bsky.feed.post/3l3bobreply", Reason: reason, ReasonSubject: subject, Record: json.RawMessage(record), IndexedAt: at}
		n.Author.DID, n.Author.Handle, n.Author.DisplayName = bobDID, "bob.bsky.social", "Bob"
		return n
	}
	now := time.Now().UTC()
	at := func(seconds int) string {
		return now.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339Nano)
	}
	w.writer.told = []bsky.Notification{
		told("like", "at://"+anaDID+"/app.bsky.feed.post/3l3elsewhere", `{}`, at(5)),
		told("follow", "", `{}`, at(4)),
		told("reply", "", `{"text":"Welcome","reply":{"root":{"uri":"`+mine+`"},"parent":{"uri":"`+mine+`"}}}`, at(3)),
		told("repost", mine, `{}`, at(2)),
		told("like", mine, `{}`, at(1)),
		// Told of before the account was connected: not gone back for.
		told("like", mine, `{}`, now.Add(-time.Hour).Format(time.RFC3339Nano)),
	}
	w.accounts.kept[anaID].NotifiedAt = now

	w.h.CheckReactions()

	bob := w.keptAt("at://" + bobDID)
	if bob == nil {
		t.Fatal("whoever did it was not kept")
	}
	if !graph.likes[fmt.Sprint(bob.ID, ">", 80)] || len(graph.likes) != 1 {
		t.Errorf("likes %v", graph.likes)
	}
	if !w.reposts.made[fmt.Sprint(bob.ID, ">", 80)] {
		t.Errorf("reposts %v", w.reposts.made)
	}
	if !graph.follows[fmt.Sprint(bob.ID, ">", anaID)] {
		t.Errorf("follows %v", graph.follows)
	}
	reply := w.keptAt("at://" + bobDID + "/app.bsky.feed.post/3l3bobreply")
	if reply == nil || reply.ParentID != 80 || reply.OwnerID != bob.ID || reply.Body != "Welcome" {
		t.Errorf("the reply was kept as %+v", reply)
	}
	// Ana is told of the like and the repost as of anybody's.
	if got := strings.Join(w.said, "|"); !strings.Contains(got, "edge.voteup.text.added.v1 80") || !strings.Contains(got, "node.repost.repost.note.added.v1") {
		t.Errorf("said %q", got)
	}

	// Asked again, nothing is done twice.
	before := len(w.nodes.kept)
	w.said = nil
	w.h.CheckReactions()
	if len(w.nodes.kept) != before || len(w.said) != 0 {
		t.Errorf("done again: %d kept, said %v", len(w.nodes.kept), w.said)
	}

	// Somebody Ana has blocked does none of it.
	w.bluesky.profiles[aliceDID] = nil
	graph.blocked[fmt.Sprint(anaID, ">", bob.ID)] = true
	graph.likes = map[string]bool{}
	w.accounts.kept[anaID].NotifiedAt = now
	w.h.CheckReactions()
	if len(graph.likes) != 0 {
		t.Errorf("a like by somebody blocked was kept: %v", graph.likes)
	}
}

// fakeMedia makes and removes posts as the graph does for somebody here.
type fakeMedia struct {
	services.MediaGRPCInterface
	nodes *fakeNodes
	// The replies hidden from their threads.
	hidden map[int64]bool
}

func (f *fakeMedia) Thread(rootID int64, _ *entities.Viewer) ([]*entities.AnahitaNode, error) {
	var replies []*entities.AnahitaNode
	for _, node := range f.nodes.kept {
		if node.RootID == rootID {
			c := *node
			c.Hidden = f.hidden[node.ID]
			replies = append(replies, &c)
		}
	}
	return replies, nil
}

func (f *fakeMedia) Add(medium *entities.AnahitaNode, _ *entities.Viewer) (*entities.AnahitaNode, error) {
	f.nodes.nextID++
	made := *medium
	made.ID = 1000 + f.nodes.nextID
	f.nodes.kept = append(f.nodes.kept, &made)
	c := made
	return &c, nil
}

func (f *fakeMedia) Delete(id int64, _ *entities.Viewer) error {
	var left []*entities.AnahitaNode
	for _, node := range f.nodes.kept {
		if node.ID != id {
			left = append(left, node)
		}
	}
	f.nodes.kept = left
	return nil
}

// THE RULE THIS GUARDS: what somebody posts on Bluesky with the account
// they connected is theirs here, once, and is not sent back there.
func TestWhatIsPostedOnBlueskyWithOnesOwnAccount(t *testing.T) {
	w, _ := writingWorld(t)
	w.h.Media = &fakeMedia{nodes: w.nodes}
	w.nodes.kept[0].Enabled = true
	for _, node := range w.nodes.kept {
		if node.ID == anaID {
			node.Enabled = true
		}
	}
	w.connect()
	ctx := context.Background()

	if wanted, _ := w.h.Wanted(); !slices.Contains(wanted, anaDID) || !slices.Contains(wanted, aliceDID) {
		t.Fatalf("listened for: %v", wanted)
	}

	heard := func(operation, rkey, record string) {
		t.Helper()
		w.h.Heard(ctx, &bsky.Event{DID: anaDID, Kind: "commit", Commit: &struct {
			Operation  string          `json:"operation"`
			Collection string          `json:"collection"`
			RKey       string          `json:"rkey"`
			Record     json.RawMessage `json:"record"`
		}{operation, bsky.PostCollection, rkey, json.RawMessage(record)}})
	}
	mine := func() []*entities.AnahitaNode {
		var posts []*entities.AnahitaNode
		for _, node := range w.nodes.kept {
			if node.CreatedBy == anaID && node.ID > 1000 {
				posts = append(posts, node)
			}
		}
		return posts
	}

	heard("create", "3l3own", `{"text":"Written on Bluesky #beans","langs":["en"]}`)
	heard("create", "3l3own", `{"text":"Written on Bluesky #beans","langs":["en"]}`)
	posts := mine()
	if len(posts) != 1 {
		t.Fatalf("%d posts kept", len(posts))
	}
	post := posts[0]
	if post.Type != constants.NodeMediumNoteEntityType || post.OwnerID != anaID || post.Access != constants.AnahitaAccessPublic ||
		post.Body != "Written on Bluesky #beans" || post.IsRemote() || !markedOwn(post.Meta) {
		t.Fatalf("kept as %+v", post)
	}
	// Nothing of Ana is kept as somebody elsewhere.
	if w.keptAt("at://"+anaDID) != nil {
		t.Error("she was kept as somebody of another server")
	}

	// That it was made here is said on the bus, as for any post: it is
	// not sent back to Bluesky.
	w.heard("node.medium.text-service.note.added.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: post.ID})
	if len(w.queue.waiting) != 0 {
		t.Fatal("a post from Bluesky was to be written to Bluesky")
	}
	// Liked here, the like reaches it there.
	w.heard("edge.voteup.text.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if failed := w.drain(); len(failed) != 0 || len(w.writer.written) != 1 || w.writer.written[0] != "create app.bsky.feed.like" {
		t.Errorf("a like of it: %v, %v", failed, w.writer.written)
	}

	// A reply of hers on Bluesky is not kept this way, nor what she wrote
	// from here and Bluesky tells of.
	heard("create", "3l3reply", `{"text":"An answer","reply":{"parent":{"uri":"at://x"}}}`)
	if len(mine()) != 1 {
		t.Errorf("%d posts kept after a reply", len(mine()))
	}

	// Deleted on Bluesky, it goes from here, and nothing is asked of
	// Bluesky about it.
	w.writer.written = nil
	heard("delete", "3l3own", ``)
	w.heard("node.medium.text-service.note.deleted.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: post.ID})
	if len(mine()) != 0 || len(w.queue.waiting) != 0 {
		t.Errorf("after it was deleted there: %d here, %d waiting", len(mine()), len(w.queue.waiting))
	}
}

// THE RULE THIS GUARDS: following here whoever one follows on Bluesky
// follows each of them once, here, and leaves out whoever is refused or
// blocked.
func TestFollowingHereWhoeverOneFollowsOnBluesky(t *testing.T) {
	w, _ := writingWorld(t)
	graph := &fakeGraph{likes: map[string]bool{}, follows: map[string]bool{}, blocked: map[string]bool{}}
	w.h.FollowEdges, w.h.Relations = fakeFollows{fakeGraph: graph}, fakeRelations{fakeGraph: graph}
	w.h.Blocks = fakeBlocks{"refused.example": true}
	w.connect()
	account := w.h.connected(anaID)

	const carolDID = "did:plc:dddddddddddddddddddddddd"
	alice, _ := w.actors.ReadByDID(aliceDID)
	graph.follows[fmt.Sprint(anaID, ">", alice.NodeID)] = true
	w.bluesky.following = map[string][]bsky.Profile{
		anaDID + "|": {
			{DID: aliceDID, Handle: "alice.bsky.social"},
			{DID: bobDID, Handle: "bob.bsky.social", DisplayName: "Bob"},
			{DID: "did:plc:eeeeeeeeeeeeeeeeeeeeeeee", Handle: "eve.refused.example"},
			{DID: "not-a-did", Handle: "x.example"},
		},
		anaDID + "|next": {{DID: carolDID, Handle: "carol.bsky.social"}},
	}

	first, next, _ := w.bluesky.Following(context.Background(), anaDID, "")
	w.h.importFollows(account, first, next, importMost)

	bob, carol := w.keptAt("at://"+bobDID), w.keptAt("at://"+carolDID)
	if bob == nil || carol == nil {
		t.Fatal("the accounts followed there were not kept")
	}
	// Alice was followed already; the other two are now. Three in all.
	if !graph.follows[fmt.Sprint(anaID, ">", bob.ID)] || !graph.follows[fmt.Sprint(anaID, ">", carol.ID)] || len(graph.follows) != 3 {
		t.Errorf("follows %v", graph.follows)
	}
	if w.keptAt("at://did:plc:eeeeeeeeeeeeeeeeeeeeeeee") != nil {
		t.Error("somebody on a refused server was kept")
	}

	// No more than was asked for: the first two of the list, and Carol,
	// further down it, is left.
	graph.follows = map[string]bool{}
	w.h.importFollows(account, first, next, 2)
	if len(graph.follows) != 2 || graph.follows[fmt.Sprint(anaID, ">", carol.ID)] {
		t.Errorf("asked for the first two: %v", graph.follows)
	}
	graph.follows[fmt.Sprint(anaID, ">", carol.ID)] = true

	// Asked again, nobody is followed twice.
	w.h.importFollows(account, first, next, importMost)
	if len(graph.follows) != 3 {
		t.Errorf("follows after a second time: %v", graph.follows)
	}
}

// fakeKept is what is kept here of what people on Bluesky did, and
// fakeNow what Bluesky says is so now.
type fakeKept struct {
	likes, reposts []*repositories.KeptReaction
	replies        []*repositories.KeptReply
	follows        []*repositories.KeptFollow
}

func (f *fakeKept) Likes(int) ([]*repositories.KeptReaction, error)   { return f.likes, nil }
func (f *fakeKept) Reposts(int) ([]*repositories.KeptReaction, error) { return f.reposts, nil }
func (f *fakeKept) Replies(int) ([]*repositories.KeptReply, error)    { return f.replies, nil }
func (f *fakeKept) Follows(int) ([]*repositories.KeptFollow, error)   { return f.follows, nil }

type fakeNow struct {
	likedBy, repostedBy map[string]bool
	following           map[string]bool
	// A list too long to be read to its end, and a reader that is away.
	partial bool
	err     error
	asked   int
}

func (f *fakeNow) LikedBy(context.Context, string) (map[string]bool, bool, error) {
	f.asked++
	return f.likedBy, !f.partial, f.err
}

func (f *fakeNow) RepostedBy(context.Context, string) (map[string]bool, bool, error) {
	f.asked++
	return f.repostedBy, !f.partial, f.err
}

func (f *fakeNow) IsFollowing(_ context.Context, did, other string) (bool, error) {
	return f.following[did], f.err
}

// THE RULE THIS GUARDS: what somebody on Bluesky took back is removed
// here, and nothing is removed on a doubt.
func TestWhatPeopleOnBlueskyTookBack(t *testing.T) {
	w, _ := writingWorld(t)
	graph := &fakeGraph{likes: map[string]bool{}, follows: map[string]bool{}, blocked: map[string]bool{}}
	w.h.Likes, w.h.FollowEdges, w.h.Relations = fakeLikes{fakeGraph: graph}, fakeFollows{fakeGraph: graph}, fakeRelations{fakeGraph: graph}
	w.connect()

	// A note of Ana's that went out, which Alice liked, reposted, answered,
	// and then followed Ana.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 80, Type: constants.NodeMediumNoteEntityType, Service: "text-service", Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Anahita to Bluesky",
	})
	alice, _ := w.actors.ReadByDID(aliceDID)
	answer := bsky.PostURI(aliceDID, "3l3answer")
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 81, Type: constants.NodeMediumNoteEntityType, URI: answer, OwnerID: alice.NodeID, ParentID: 80, Body: "Welcome",
	})
	set := func() {
		graph.likes[fmt.Sprint(alice.NodeID, ">", 80)] = true
		graph.follows[fmt.Sprint(alice.NodeID, ">", anaID)] = true
		w.reposts.made[fmt.Sprint(alice.NodeID, ">", 80)] = true
	}
	set()
	reaction := &repositories.KeptReaction{ActorID: alice.NodeID, ActorDID: aliceDID, PostID: 80, PostDID: anaDID, PostRKey: "3l3mine"}
	w.h.Kept = &fakeKept{
		likes:   []*repositories.KeptReaction{reaction, reaction},
		reposts: []*repositories.KeptReaction{reaction},
		replies: []*repositories.KeptReply{{ActorID: alice.NodeID, URI: answer}},
		follows: []*repositories.KeptFollow{{ActorID: alice.NodeID, ActorDID: aliceDID, PersonID: anaID, PersonDID: anaDID}},
	}
	all := func() string {
		return fmt.Sprint(len(graph.likes), len(w.reposts.made), len(graph.follows), w.keptAt(answer) != nil)
	}

	// All of it is still so on Bluesky: nothing goes.
	now := &fakeNow{
		likedBy: map[string]bool{aliceDID: true}, repostedBy: map[string]bool{aliceDID: true},
		following: map[string]bool{aliceDID: true},
	}
	w.bluesky.posts[answer] = aPost(t, `{"text":"Welcome"}`)
	w.h.Verify = now
	w.h.VerifyReactions()
	if got := all(); got != "1 1 1 true" {
		t.Fatalf("with nothing taken back: %s", got)
	}
	// Each post is asked about once, however many liked it.
	if now.asked != 2 {
		t.Errorf("asked %d times", now.asked)
	}

	// The reader is away, or the lists are too long to read to the end:
	// nothing goes, though nobody is named in them.
	now.likedBy, now.repostedBy, now.following = map[string]bool{}, map[string]bool{}, map[string]bool{}
	delete(w.bluesky.posts, answer)
	w.bluesky.err = errors.New("timeout")
	now.err = errors.New("timeout")
	w.h.VerifyReactions()
	now.err, now.partial = nil, true
	w.bluesky.err = nil
	w.bluesky.posts[answer] = aPost(t, `{"text":"Welcome"}`)
	now.following = map[string]bool{aliceDID: true}
	w.h.VerifyReactions()
	if got := all(); got != "1 1 1 true" {
		t.Fatalf("on a doubt: %s", got)
	}

	// Taken back, all four.
	now.partial = false
	now.following = map[string]bool{}
	delete(w.bluesky.posts, answer)
	w.said = nil
	w.h.VerifyReactions()
	if got := all(); got != "0 0 0 false" {
		t.Fatalf("after all of it was taken back: %s", got)
	}
	if got := strings.Join(w.said, "|"); !strings.Contains(got, "edge.voteup.text.deleted.v1 80") || !strings.Contains(got, event.RemoteRepostDeleted) {
		t.Errorf("said %q", got)
	}
}

// THE RULE THIS GUARDS: nothing leaves for Bluesky from a site kept to its
// members, or for somebody whose profile not everybody may see, as for the
// fediverse.
func TestNothingIsWrittenFromWhatIsNotPublic(t *testing.T) {
	w, post := writingWorld(t)
	w.connect()

	// Ana's profile is kept to her followers: her like stays here.
	w.nodes.hidden = map[int64]bool{anaID: true}
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if len(w.queue.waiting) != 0 || len(w.sent.rows) != 0 {
		t.Fatal("something was written for somebody whose profile is not public")
	}

	// Open again, but the site is kept to its members: still nothing, and
	// connecting is not offered.
	w.nodes.hidden = nil
	restore := policy.SetReadAccessForTest(policy.ReadRegistered)
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if len(w.queue.waiting) != 0 || len(w.sent.rows) != 0 {
		t.Fatal("something was written from a members-only site")
	}
	if w.h.canWrite() {
		t.Error("connecting is offered on a members-only site")
	}

	// An open site and a public profile: the like goes.
	restore()
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: post.ID})
	if len(w.sent.rows) != 1 {
		t.Errorf("the like was not written: %d", len(w.sent.rows))
	}
}

// THE RULE THIS GUARDS: blocking somebody on Bluesky here blocks them on
// Bluesky too, for somebody who connected their account, and unblocking
// takes back the block that was written from here.
func TestABlockIsToldToBluesky(t *testing.T) {
	w, _ := writingWorld(t)
	alice, _ := w.actors.ReadByDID(aliceDID)
	block := func(name string) {
		w.heard(name, event.EventPayload{ObjectID: anaID, TargetID: alice.NodeID})
	}

	// Not connected: the block is here and nowhere else.
	block("edge.block.socialgraph.added.v1")
	if len(w.queue.waiting) != 0 {
		t.Fatal("a block was written for somebody who connected nothing")
	}

	w.connect()
	block("edge.block.socialgraph.added.v1")
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if got := strings.Join(w.writer.written, ", "); got != "create app.bsky.graph.block" {
		t.Fatalf("written: %s", got)
	}
	out, _ := json.Marshal(w.writer.records["app.bsky.graph.block/"+w.sent.rows[0].RKey])
	if !strings.Contains(string(out), `"$type":"app.bsky.graph.block"`) || !strings.Contains(string(out), `"subject":"`+aliceDID+`"`) {
		t.Errorf("the block as written: %s", out)
	}

	// Heard twice, written once.
	block("edge.block.socialgraph.added.v1")
	if len(w.queue.waiting) != 0 {
		t.Errorf("a block was written twice")
	}

	// Unblocked: taken back there.
	block("edge.block.socialgraph.deleted.v1")
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	if got := strings.Join(w.writer.written, ", "); got != "create app.bsky.graph.block, delete app.bsky.graph.block" {
		t.Errorf("written: %s", got)
	}
}

// THE RULE THIS GUARDS: a reply on Bluesky hidden here, under a post that
// went there, is hidden there too, by the post's threadgate; and only that
// reply is changed in it, so what was set in Bluesky's own app stays.
func TestAHiddenReplyIsHiddenOnBluesky(t *testing.T) {
	w, _ := writingWorld(t)
	media := &fakeMedia{nodes: w.nodes, hidden: map[int64]bool{}}
	w.h.Media = media
	w.connect()

	// A note of Ana's that went to Bluesky.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{
		ID: 80, Type: constants.NodeMediumNoteEntityType, Service: "text-service", Enabled: true, OwnerID: anaID, CreatedBy: anaID,
		Access: constants.AnahitaAccessPublic, Body: "Anahita to Bluesky",
	})
	w.heard("node.medium.text-service.note.added.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: 80})
	if failed := w.drain(); len(failed) != 0 {
		t.Fatal(failed)
	}
	rkey := w.sent.rows[0].RKey
	gateAt := bsky.ThreadgateCollection + "/" + rkey

	// Under it: Alice's answer, from Bluesky, and one from somebody here
	// that never went there.
	alice, _ := w.actors.ReadByDID(aliceDID)
	answer := bsky.PostURI(aliceDID, "3l3answer")
	w.nodes.kept = append(w.nodes.kept,
		&entities.AnahitaNode{ID: 81, Type: constants.NodeMediumNoteEntityType, URI: answer, OwnerID: alice.NodeID, CreatedBy: alice.NodeID, ParentID: 80, RootID: 80, Body: "Welcome"},
		&entities.AnahitaNode{ID: 82, Type: constants.NodeMediumNoteEntityType, OwnerID: 9, CreatedBy: 9, ParentID: 80, RootID: 80, Body: "Local", Enabled: true},
	)
	// Made in Bluesky's own app: only followers may reply, and a reply not
	// kept here is hidden.
	elsewhere := bsky.PostURI(bobDID, "3l3elsewhere")
	w.writer.records[gateAt] = map[string]any{
		"$type": bsky.ThreadgateCollection, "post": bsky.PostURI(anaDID, rkey), "createdAt": "2026-10-01T00:00:00.000Z",
		"allow":         []any{map[string]any{"$type": "app.bsky.feed.threadgate#followerRule"}},
		"hiddenReplies": []any{elsewhere},
	}
	hide := func(reply int64, hidden bool) {
		t.Helper()
		media.hidden[reply] = hidden
		name := "edge.hidden-reply.medium.deleted.v1"
		if hidden {
			name = "edge.hidden-reply.medium.added.v1"
		}
		w.heard(name, event.EventPayload{SubjectID: anaID, ObjectID: reply, TargetID: 80})
		if failed := w.drain(); len(failed) != 0 {
			t.Fatal(failed)
		}
	}
	gate := func() string {
		out, _ := json.Marshal(w.writer.records[gateAt])
		return string(out)
	}

	// The reply from Bluesky, hidden: added, and the rest left as it was.
	hide(81, true)
	got := gate()
	for _, want := range []string{`"hiddenReplies":["` + elsewhere + `","` + answer + `"]`, `followerRule`, `"post":"` + bsky.PostURI(anaDID, rkey) + `"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the threadgate lacks %s:\n%s", want, got)
		}
	}

	// Heard twice, nothing more is written.
	written := len(w.writer.written)
	hide(81, true)
	// A reply that is not on Bluesky: nothing is written.
	hide(82, true)
	if len(w.writer.written) != written {
		t.Errorf("written again: %v", w.writer.written[written:])
	}

	// Shown again: taken out, and what Bluesky's app hid stays.
	hide(81, false)
	if got := gate(); !strings.Contains(got, `"hiddenReplies":["`+elsewhere+`"]`) || !strings.Contains(got, `followerRule`) {
		t.Errorf("after showing it again: %s", got)
	}

	// Hidden and shown again before it was written: what is so at the end
	// is written, which is nothing to change.
	written = len(w.writer.written)
	media.hidden[81] = true
	w.heard("edge.hidden-reply.medium.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: 81, TargetID: 80})
	media.hidden[81] = false
	w.heard("edge.hidden-reply.medium.deleted.v1", event.EventPayload{SubjectID: anaID, ObjectID: 81, TargetID: 80})
	if failed := w.drain(); len(failed) != 0 || len(w.writer.written) != written {
		t.Errorf("hidden and shown before it was written: %v, %v", failed, w.writer.written[written:])
	}

	// A post with no threadgate gets one, saying nothing of who may reply.
	delete(w.writer.records, gateAt)
	hide(81, true)
	if got := gate(); !strings.Contains(got, `"hiddenReplies":["`+answer+`"]`) || strings.Contains(got, `"allow"`) {
		t.Errorf("a new threadgate: %s", got)
	}

	// The post deleted here: its threadgate goes with it.
	w.heard("node.medium.text-service.note.deleted.v1", event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType, ObjectID: 80})
	if failed := w.drain(); len(failed) != 0 || len(w.writer.records) != 0 {
		t.Errorf("after the post was deleted: %v, %v", failed, w.writer.records)
	}
}

// THE RULE THIS GUARDS: hiding a reply under a post that never went to
// Bluesky, or somebody else's, writes nothing there.
func TestAHiddenReplyUnderAPostNotOnBluesky(t *testing.T) {
	w, post := writingWorld(t)
	w.h.Media = &fakeMedia{nodes: w.nodes, hidden: map[int64]bool{}}
	w.connect()

	// Alice's post, kept from Bluesky: hers to hide replies under there.
	w.heard("edge.hidden-reply.medium.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: 99, TargetID: post.ID})
	// A note here that went nowhere.
	w.nodes.kept = append(w.nodes.kept, &entities.AnahitaNode{ID: 70, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID, Body: "Here only"})
	w.heard("edge.hidden-reply.medium.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: 99, TargetID: 70})
	if len(w.queue.waiting) != 0 {
		t.Errorf("%d things wait to be written", len(w.queue.waiting))
	}
}
