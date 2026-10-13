package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/event"
	"anahita-libs/federation"
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
	now := f.kept[id].BrokenAt
	if now == nil {
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
	access  string
	refresh string
	written []string
	records map[string]any
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

func (f *fakeWriter) RefOf(_ context.Context, uri string) (*bsky.Ref, *bsky.Ref, error) {
	if strings.Contains(uri, "gone") {
		return nil, nil, bsky.ErrNobody
	}
	return &bsky.Ref{URI: uri, CID: "cid-of-" + uri[len(uri)-6:]}, &bsky.Ref{URI: "at://root", CID: "cid-root"}, nil
}

type fakeQueue struct {
	waiting []federation.Delivery
	keys    map[string]bool
}

func (f *fakeQueue) Enqueue(_ context.Context, actorID int64, target string, payload []byte, key string) error {
	if f.keys[key] {
		return nil
	}
	f.keys[key] = true
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
		// A note of Ana's that answers nothing.
		&entities.AnahitaNode{ID: 72, Type: constants.NodeMediumNoteEntityType, Enabled: true, OwnerID: anaID, CreatedBy: anaID, Body: "A note of my own"},
	)
	w.heard("edge.voteup.note.added.v1", event.EventPayload{SubjectID: anaID, ObjectID: 70})
	note := event.EventPayload{ObjectType: constants.NodeMediumNoteEntityType}
	for _, id := range []int64{71, 72} {
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
