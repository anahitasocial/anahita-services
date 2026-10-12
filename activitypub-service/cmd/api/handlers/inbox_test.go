package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"activitypub/data/repositories"
	"activitypub/fetch"
	"activitypub/httpsig"
	"activitypub/keys"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/federation"

	"github.com/labstack/echo/v4"
)

type fakeEdges struct {
	services.SocialgraphFollowGRPCInterface
	log []string
}

func (f *fakeEdges) Add(req *services.SocialgraphFollowAddRequest) error {
	kind := "follow"
	if req.IsFollowRequest {
		kind = "request"
	}
	f.log = append(f.log, "add "+kind+" "+req.Source.Alias+">"+req.Target.Alias)
	return nil
}

func (f *fakeEdges) Delete(req *services.SocialgraphFollowDeleteRequest) error {
	kind := "follow"
	if req.IsFollowRequest {
		kind = "request"
	}
	f.log = append(f.log, "delete "+kind+" "+req.Source.Alias+">"+req.Target.Alias)
	return nil
}

type fakeRelations struct {
	services.SocialgraphGRPCInterface
	relation *entities.ActorRelation
}

func (f *fakeRelations) Read(a, b int64) (*entities.ActorRelation, error) {
	if f.relation == nil {
		return &entities.ActorRelation{ActorAId: a, ActorBId: b}, nil
	}
	return f.relation, nil
}

type queued struct {
	actorID int64
	target  string
	payload map[string]any
	key     string
}

type fakeQueue struct{ sent []queued }

func (f *fakeQueue) Enqueue(_ context.Context, actorID int64, target string, payload []byte, key string) error {
	for _, have := range f.sent {
		if have.key == key {
			return nil
		}
	}
	var read map[string]any
	_ = json.Unmarshal(payload, &read)
	f.sent = append(f.sent, queued{actorID, target, read, key})
	return nil
}

type fakeFollows struct {
	kept map[string]*repositories.Follow
	// Where a person's followers elsewhere are written to.
	inboxes []string
}

func (f *fakeFollows) FollowerInboxes(int64) ([]string, error) { return f.inboxes, nil }
func (f *fakeFollows) IsFollowedFromHere(remoteID int64) (bool, error) {
	for _, follow := range f.kept {
		if follow.RemoteID == remoteID && follow.Direction == repositories.FollowOut {
			return true, nil
		}
	}
	return false, nil
}

type fakeObjects struct {
	kept map[int64]*repositories.Object
}

func (f *fakeObjects) Begin(object *repositories.Object) (bool, error) {
	if f.kept[object.NodeID] != nil {
		return false, nil
	}
	f.kept[object.NodeID] = object
	return true, nil
}
func (f *fakeObjects) Read(id int64) (*repositories.Object, error) { return f.kept[id], nil }
func (f *fakeObjects) End(id int64) (bool, error) {
	_, was := f.kept[id]
	delete(f.kept, id)
	return was, nil
}
func (f *fakeObjects) Count(actorID int64) (int64, error) {
	var n int64
	for _, object := range f.kept {
		if object.ActorID == actorID {
			n++
		}
	}
	return n, nil
}

// DeleteRemote as the graph does it: only something from elsewhere, and
// only its owner's.
func (f *fakeNodes) DeleteRemote(uri string, ownerID int64) (bool, error) {
	for i, kept := range f.people {
		if kept.URI == uri && uri != "" && kept.OwnerID == ownerID {
			f.people = append(f.people[:i], f.people[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func followKey(l, r int64, d string) string { return string(rune(l)) + "|" + string(rune(r)) + "|" + d }

func (f *fakeFollows) Save(follow *repositories.Follow) error {
	f.kept[followKey(follow.LocalID, follow.RemoteID, follow.Direction)] = follow
	return nil
}
func (f *fakeFollows) Begin(follow *repositories.Follow) (bool, error) {
	key := followKey(follow.LocalID, follow.RemoteID, follow.Direction)
	if f.kept[key] != nil {
		return false, nil
	}
	f.kept[key] = follow
	return true, nil
}
func (f *fakeFollows) ReadByActivity(id string) (*repositories.Follow, error) {
	for _, follow := range f.kept {
		if follow.ActivityID == id {
			return follow, nil
		}
	}
	return nil, nil
}
func (f *fakeFollows) Read(l, r int64, d string) (*repositories.Follow, error) {
	return f.kept[followKey(l, r, d)], nil
}
func (f *fakeFollows) Delete(l, r int64, d string) error {
	delete(f.kept, followKey(l, r, d))
	return nil
}

type inboxWorld struct {
	*world
	fetcher   *fakeFetcher
	actors    *fakeActors
	edges     *fakeEdges
	relations *fakeRelations
	queue     *fakeQueue
	follows   *fakeFollows
	objects   *fakeObjects
}

func newInboxWorld() *inboxWorld {
	w, fetcher, actors := lookupWorld()
	iw := &inboxWorld{
		world: w, fetcher: fetcher, actors: actors,
		edges: &fakeEdges{}, relations: &fakeRelations{}, queue: &fakeQueue{},
		follows: &fakeFollows{kept: map[string]*repositories.Follow{}},
		objects: &fakeObjects{kept: map[int64]*repositories.Object{}},
	}
	w.h.FollowEdges, w.h.Relations, w.h.Queue, w.h.Follows = iw.edges, iw.relations, iw.queue, iw.follows
	w.h.Objects = iw.objects
	return iw
}

const (
	anaActor = "https://api.example.org/ap/users/" + anaUUID
	followID = "https://mastodon.example/activities/follow-1"
)

func follow() string {
	return `{"@context":"https://www.w3.org/ns/activitystreams","id":"` + followID + `","type":"Follow","actor":"` + aliceID + `","object":"` + anaActor + `"}`
}

func undo() string {
	return `{"id":"https://mastodon.example/activities/undo-1","type":"Undo","actor":"` + aliceID + `","object":` + follow() + `}`
}

// deliver posts to the inbox as another server would: signed with a key,
// under a key's name.
func (w *inboxWorld) deliver(body, keyID, privatePEM string, change func(*http.Request)) *httptest.ResponseRecorder {
	out, _ := http.NewRequest(http.MethodPost, "https://api.example.org/inbox", strings.NewReader(body))
	if privatePEM != "" {
		private, err := keys.ParsePrivate(privatePEM)
		if err != nil {
			panic(err)
		}
		if err := httpsig.Sign(out, []byte(body), keyID, private, time.Now()); err != nil {
			panic(err)
		}
	}

	in := httptest.NewRequest(http.MethodPost, "/inbox", bytes.NewReader([]byte(body)))
	in.Host = "api.example.org"
	in.Header = out.Header.Clone()
	if change != nil {
		change(in)
	}

	rec := httptest.NewRecorder()
	_ = w.h.Inbox(echo.New().NewContext(in, rec))
	return rec
}

func (w *inboxWorld) fromAlice(body string) *httptest.ResponseRecorder {
	return w.deliver(body, aliceID+"#main-key", remoteKey.PrivatePEM, nil)
}

// THE RULE THIS GUARDS: a follow from another server is acted on when, and
// only when, the person who is said to have followed is the one who signed.
func TestAFollowFromElsewhereIsAcceptedAndAnswered(t *testing.T) {
	w := newInboxWorld()

	if rec := w.fromAlice(follow()); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}

	// She is kept, she follows, and it is recorded as the other server
	// calls it.
	if len(w.actors.kept) != 1 || len(w.edges.log) != 1 || w.edges.log[0] != "add follow alice@mastodon.example>ana" {
		t.Fatalf("kept %d, edges %v", len(w.actors.kept), w.edges.log)
	}
	var saved *repositories.Follow
	for _, f := range w.follows.kept {
		saved = f
	}
	if saved == nil || saved.LocalID != anaID || saved.Direction != repositories.FollowIn || saved.ActivityID != followID || saved.State != repositories.FollowAccepted {
		t.Errorf("recorded %+v", saved)
	}

	// She is told yes: to her inbox, in ana's name, quoting what she sent.
	if len(w.queue.sent) != 1 {
		t.Fatalf("queued %d", len(w.queue.sent))
	}
	sent := w.queue.sent[0]
	object, _ := sent.payload["object"].(map[string]any)
	if sent.actorID != anaID || sent.target != aliceID+"/inbox" || sent.payload["type"] != "Accept" || sent.payload["actor"] != anaActor ||
		object["id"] != followID || object["type"] != "Follow" || object["actor"] != aliceID || object["object"] != anaActor ||
		!strings.HasPrefix(sent.payload["id"].(string), "https://api.example.org/ap/activities/") {
		t.Errorf("sent %+v", sent)
	}

	// Sent again, as servers do: she is told yes once more at most, and
	// does not follow twice.
	w.relations.relation = &entities.ActorRelation{IsLeading: true}
	if rec := w.fromAlice(follow()); rec.Code != http.StatusAccepted {
		t.Fatalf("again: %d", rec.Code)
	}
	if len(w.edges.log) != 1 || len(w.queue.sent) != 1 {
		t.Errorf("after the second: edges %v, queued %d", w.edges.log, len(w.queue.sent))
	}
}

func TestWhatIsNotSignedByWhoItSaysIsRefused(t *testing.T) {
	mallory, _ := keys.Generate()

	cases := map[string]func(w *inboxWorld) *httptest.ResponseRecorder{
		"unsigned": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), "", "", nil)
		},
		"signed with another key under her name": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), aliceID+"#main-key", mallory.PrivatePEM, nil)
		},
		"signed by somebody who is not there": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), "https://mastodon.example/users/mallory#main-key", mallory.PrivatePEM, nil)
		},
		"signed by her, saying somebody else did it": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.fromAlice(strings.Replace(follow(), `"actor":"`+aliceID+`"`, `"actor":"https://mastodon.example/users/bob"`, 1))
		},
		"signed in this installation's name": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), anaActor+"#main-key", mallory.PrivatePEM, nil)
		},
		"a body other than the one signed": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), aliceID+"#main-key", remoteKey.PrivatePEM, func(r *http.Request) {
				r.Body = http.NoBody
				swapped := strings.Replace(follow(), "follow-1", "follow-2", 1)
				r.Body = readCloser(swapped)
			})
		},
		"sent to another host": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), aliceID+"#main-key", remoteKey.PrivatePEM, func(r *http.Request) { r.Host = "other.example" })
		},
		"signed long ago": func(w *inboxWorld) *httptest.ResponseRecorder {
			return w.deliver(follow(), aliceID+"#main-key", remoteKey.PrivatePEM, func(r *http.Request) {
				r.Header.Set("Date", time.Now().Add(-48*time.Hour).UTC().Format(http.TimeFormat))
			})
		},
	}

	for name, send := range cases {
		w := newInboxWorld()
		rec := send(w)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d", name, rec.Code)
		}
		if len(w.edges.log) != 0 || len(w.queue.sent) != 0 || len(w.actors.kept) != 0 {
			t.Errorf("%s: something was done: edges %v, queued %d, kept %d", name, w.edges.log, len(w.queue.sent), len(w.actors.kept))
		}
	}

	// Not readable at all.
	w := newInboxWorld()
	if rec := w.fromAlice(`not json`); rec.Code != http.StatusBadRequest {
		t.Errorf("not json: %d", rec.Code)
	}
	if rec := w.fromAlice(`{"type":"Follow","actor":"` + aliceID + `","pad":"` + strings.Repeat("x", inboxMaxBytes) + `"}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("too large: %d", rec.Code)
	}
}

func readCloser(body string) *bodyReader { return &bodyReader{Reader: strings.NewReader(body)} }

type bodyReader struct{ *strings.Reader }

func (*bodyReader) Close() error { return nil }

func TestWhoIsFollowedDecidesWhatHappens(t *testing.T) {
	// Letting people ask to follow is for a profile that is not open to
	// everybody. One that is, is followed at once, from elsewhere as from
	// here.
	w := newInboxWorld()
	w.nodes.people[0].AllowFollowRequest = true
	w.fromAlice(follow())
	if len(w.edges.log) != 1 || w.edges.log[0] != "add follow alice@mastodon.example>ana" || len(w.queue.sent) != 1 {
		t.Errorf("a profile that lets people ask: edges %v, queued %d", w.edges.log, len(w.queue.sent))
	}

	// Somebody the person here has blocked is told no, and does not follow.
	w = newInboxWorld()
	w.relations.relation = &entities.ActorRelation{IsBlocking: true}
	w.fromAlice(follow())
	if len(w.edges.log) != 0 || len(w.queue.sent) != 1 || w.queue.sent[0].payload["type"] != "Reject" {
		t.Errorf("blocked: edges %v, queued %+v", w.edges.log, w.queue.sent)
	}

	// Somebody who is not on the fediverse is not followed, and the sender
	// is told nothing either way.
	for name, change := range notThere() {
		w = newInboxWorld()
		change(w.world)
		w.nodes.people[0] = w.ana
		rec := w.fromAlice(follow())
		if (rec.Code != http.StatusAccepted && rec.Code != http.StatusNotFound) || len(w.edges.log) != 0 || len(w.queue.sent) != 0 {
			t.Errorf("%s: %d, edges %v, queued %d", name, rec.Code, w.edges.log, len(w.queue.sent))
		}
	}

	// A follow of somebody on another server altogether is not ours.
	w = newInboxWorld()
	w.fromAlice(strings.Replace(follow(), anaActor, "https://other.example/users/ana", 1))
	if len(w.edges.log) != 0 || len(w.queue.sent) != 0 {
		t.Errorf("not ours: edges %v", w.edges.log)
	}

	// Something this installation does not understand is taken and ignored.
	w = newInboxWorld()
	if rec := w.fromAlice(`{"id":"x","type":"Dance","actor":"` + aliceID + `","object":"` + anaActor + `"}`); rec.Code != http.StatusAccepted || len(w.edges.log) != 0 {
		t.Errorf("a dance: %d, %v", rec.Code, w.edges.log)
	}
}

func TestAFollowIsTakenBack(t *testing.T) {
	w := newInboxWorld()
	w.fromAlice(follow())
	w.edges.log = nil

	if rec := w.fromAlice(undo()); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	if strings.Join(w.edges.log, ", ") != "delete follow alice@mastodon.example>ana, delete request alice@mastodon.example>ana" || len(w.follows.kept) != 0 {
		t.Errorf("edges %v, kept %d", w.edges.log, len(w.follows.kept))
	}

	// Somebody else's follow is not hers to take back.
	w = newInboxWorld()
	w.fromAlice(follow())
	w.edges.log = nil
	w.fromAlice(strings.Replace(undo(), `"type":"Follow","actor":"`+aliceID+`"`, `"type":"Follow","actor":"https://mastodon.example/users/bob"`, 1))
	if len(w.edges.log) != 0 {
		t.Errorf("another's follow was undone: %v", w.edges.log)
	}

	// Taking back what was never there is nothing, and no failure.
	w = newInboxWorld()
	if rec := w.fromAlice(undo()); rec.Code != http.StatusAccepted || len(w.edges.log) != 0 {
		t.Errorf("never followed: %d %v", rec.Code, w.edges.log)
	}
}

type fakePoster struct {
	posted []string
	err    error
	keyID  string
}

func (f *fakePoster) Post(_ context.Context, inbox string, body []byte, signer fetch.Signer) error {
	keyID, key, err := signer()
	if err != nil || key == nil {
		return errors.New("no key to sign with")
	}
	f.keyID = keyID
	f.posted = append(f.posted, inbox+" "+string(body))
	return f.err
}

func TestWhatIsQueuedIsSentInTheNameItWasQueuedIn(t *testing.T) {
	w := newInboxWorld()
	poster := &fakePoster{}
	w.h.Poster = poster

	delivery := federation.Delivery{ActorID: anaID, Target: aliceID + "/inbox", Payload: []byte(`{"type":"Accept"}`)}
	if err := w.h.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	if len(poster.posted) != 1 || poster.keyID != anaActor+"#main-key" {
		t.Errorf("posted %v as %s", poster.posted, poster.keyID)
	}

	// An inbox that is gone, or will never take it, is not tried again; one
	// that is down is.
	for err, permanent := range map[error]bool{fetch.ErrGone: true, fetch.ErrRefused: true, errors.New("status 503"): false} {
		poster.err = err
		got := w.h.Deliver(context.Background(), delivery)
		if got == nil || errors.Is(got, federation.ErrPermanent) != permanent {
			t.Errorf("%v: answered %v", err, got)
		}
	}

	// Nobody left to send it for.
	delivery.ActorID = 404
	if err := w.h.Deliver(context.Background(), delivery); !errors.Is(err, federation.ErrPermanent) {
		t.Errorf("nobody: %v", err)
	}
}

// Following from here: heard on the bus, asked of the other server, and
// settled by what it answers.
func TestSomebodyHereFollowsSomebodyElsewhere(t *testing.T) {
	w := newInboxWorld()

	// Alice is known here, as anybody looked up is.
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	if alice == nil {
		t.Fatal("alice was not kept")
	}

	followed := &event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: anaID, TargetID: alice.NodeID}

	// Heard twice, as every copy of the service hears it: asked once.
	for i := 0; i < 2; i++ {
		if err := w.h.HandleEvent(followed); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.queue.sent) != 1 {
		t.Fatalf("queued %d", len(w.queue.sent))
	}
	sent := w.queue.sent[0]
	asked, _ := sent.payload["id"].(string)
	if sent.actorID != anaID || sent.target != aliceID+"/inbox" || sent.payload["type"] != "Follow" ||
		sent.payload["actor"] != anaActor || sent.payload["object"] != aliceID || !strings.HasPrefix(asked, "https://api.example.org/ap/activities/") {
		t.Errorf("sent %+v", sent)
	}
	waiting, _ := w.follows.Read(anaID, alice.NodeID, repositories.FollowOut)
	if waiting == nil || waiting.State != repositories.FollowPending || waiting.ActivityID != asked {
		t.Fatalf("recorded %+v", waiting)
	}

	// Told yes by somebody else, or about a follow nobody asked for: nothing.
	accept := func(id string) string {
		return `{"id":"https://mastodon.example/a/1","type":"Accept","actor":"` + aliceID + `","object":{"id":"` + id + `","type":"Follow","actor":"` + anaActor + `","object":"` + aliceID + `"}}`
	}
	w.fromAlice(accept("https://api.example.org/ap/activities/some-other"))
	if waiting, _ = w.follows.Read(anaID, alice.NodeID, repositories.FollowOut); waiting.State != repositories.FollowPending {
		t.Error("an answer to another question was taken")
	}

	// Told yes by her, to what was asked.
	if rec := w.fromAlice(accept(asked)); rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
	if waiting, _ = w.follows.Read(anaID, alice.NodeID, repositories.FollowOut); waiting.State != repositories.FollowAccepted {
		t.Errorf("after her yes: %+v", waiting)
	}

	// Unfollowing tells her, quoting what was asked.
	if err := w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.deleted.v1", ObjectID: anaID, TargetID: alice.NodeID}); err != nil {
		t.Fatal(err)
	}
	last := w.queue.sent[len(w.queue.sent)-1]
	quoted, _ := last.payload["object"].(map[string]any)
	if last.payload["type"] != "Undo" || quoted["id"] != asked || quoted["type"] != "Follow" {
		t.Errorf("sent %+v", last)
	}
	if kept, _ := w.follows.Read(anaID, alice.NodeID, repositories.FollowOut); kept != nil {
		t.Error("the follow was still kept")
	}
}

func TestToldNoTheFollowHereIsTakenAway(t *testing.T) {
	w := newInboxWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}
	_ = w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: anaID, TargetID: alice.NodeID})
	asked, _ := w.queue.sent[0].payload["id"].(string)

	w.fromAlice(`{"id":"https://mastodon.example/a/2","type":"Reject","actor":"` + aliceID + `","object":"` + asked + `"}`)

	if len(w.edges.log) != 1 || w.edges.log[0] != "delete follow ana>alice@mastodon.example" {
		t.Errorf("edges %v", w.edges.log)
	}
	if kept, _ := w.follows.Read(anaID, alice.NodeID, repositories.FollowOut); kept != nil {
		t.Error("the follow was still kept")
	}

	// Its going is then heard on the bus, and is not taken for an unfollow.
	before := len(w.queue.sent)
	_ = w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.deleted.v1", ObjectID: anaID, TargetID: alice.NodeID})
	if len(w.queue.sent) != before {
		t.Error("an Undo was sent for a follow that was refused")
	}
}

func TestOnlySomebodyOnTheFediverseFollowsAcrossIt(t *testing.T) {
	w := newInboxWorld()
	w.call(w.h.Lookup, http.MethodGet, "/activitypub/lookup?q=@alice@mastodon.example", "", anaID)
	var alice *repositories.RemoteActor
	for _, kept := range w.actors.kept {
		alice = kept
	}

	w.accounts.enabled[anaID] = false
	_ = w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: anaID, TargetID: alice.NodeID})
	if len(w.queue.sent) != 0 || len(w.edges.log) != 1 || w.edges.log[0] != "delete follow ana>alice@mastodon.example" {
		t.Errorf("queued %d, edges %v", len(w.queue.sent), w.edges.log)
	}

	// A follow between two people here is nothing to do with this.
	w = newInboxWorld()
	_ = w.h.HandleEvent(&event.EventPayload{Name: "edge.follow.socialgraph.added.v1", ObjectID: anaID, TargetID: anaID})
	if len(w.queue.sent) != 0 || len(w.edges.log) != 0 {
		t.Errorf("a follow here: queued %d, edges %v", len(w.queue.sent), w.edges.log)
	}
}
