package handlers

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"auth/cmd/api/notifiers"
	"auth/config"
	"auth/data/entities"
	"auth/data/repositories"
	"auth/data/services"

	lib_entities "anahita-libs/data/entities"
	libservices "anahita-libs/data/services"
	"anahita-libs/sessions"

	"github.com/labstack/echo/v4"
)

// Test doubles for the auth-service handler package.
//
// Every double embeds the interface it stands in for rather than
// implementing all of it. Two reasons, and the second is the useful one:
//
//   - the interfaces are wide (UsersInterface alone has fourteen methods)
//     and a test that only resets a password should not have to spell out
//     thirteen of them;
//   - an embedded nil interface panics when a method nobody overrode is
//     called, so a handler that starts touching a new repository fails
//     loudly in the test that covers it instead of quietly passing against
//     a zero value.
//
// The panic message names the method, which is enough to find it.

// ---------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------

type fakeUsers struct {
	repositories.UsersInterface

	byEmail map[string]*entities.User
	byID    map[int64]*entities.User

	// editedPassword records what EditPassword was asked to store, so a
	// test can tell "the handler hashed a new password" from "the handler
	// wrote the old one back".
	editedPassword string
	editedID       int64
	editCalls      int

	editErr error

	onboardCalls int
	onboardErr   error
}

func newFakeUsers(users ...*entities.User) *fakeUsers {
	f := &fakeUsers{
		byEmail: map[string]*entities.User{},
		byID:    map[int64]*entities.User{},
	}
	for _, u := range users {
		f.byEmail[u.Email] = u
		f.byID[u.ID] = u
	}
	return f
}

func (f *fakeUsers) ReadByEmail(email string) (*entities.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("no user for %q", email)
}

func (f *fakeUsers) ReadByID(id int64) (*entities.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("no user for id %d", id)
}

func (f *fakeUsers) EditPassword(password string, id int64) error {
	f.editCalls++
	if f.editErr != nil {
		return f.editErr
	}
	f.editedPassword = password
	f.editedID = id
	return nil
}

// MarkOnboarded behaves as the real statement does: it stamps a user that has
// not been through onboarding and leaves one that has alone.
func (f *fakeUsers) MarkOnboarded(id int64) error {
	f.onboardCalls++
	if f.onboardErr != nil {
		return f.onboardErr
	}
	if u, ok := f.byID[id]; ok && u.OnboardedAt == nil {
		now := time.Now()
		u.OnboardedAt = &now
	}
	return nil
}

// ---------------------------------------------------------------------
// Tokens
// ---------------------------------------------------------------------

type fakeTokens struct {
	repositories.TokensInterface

	byBody map[string]*entities.Token

	added       []*entities.Token
	edited      []*entities.Token
	invalidated []string

	addErr error
}

func newFakeTokens(tokens ...*entities.Token) *fakeTokens {
	f := &fakeTokens{byBody: map[string]*entities.Token{}}
	for _, t := range tokens {
		f.byBody[t.Body] = t
	}
	return f
}

func (f *fakeTokens) ReadByBody(body string) (*entities.Token, error) {
	if t, ok := f.byBody[body]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("no token for body %q", body)
}

// InvalidateOutstanding records the call and retires anything matching, so a
// test can assert that issuing a new link kills the old one.
func (f *fakeTokens) InvalidateOutstanding(tokenType, email string) (int64, error) {
	f.invalidated = append(f.invalidated, tokenType+"/"+email)

	var retired int64
	for _, t := range f.byBody {
		if t.Type == tokenType && t.RecipientEmail == email && !t.IsUsed {
			t.IsUsed = true
			retired++
		}
	}
	return retired, nil
}

func (f *fakeTokens) Add(token *entities.Token) (*entities.Token, error) {
	if f.addErr != nil {
		return nil, f.addErr
	}
	// Validate() is what the real repository runs before the insert, so
	// run it here too — otherwise a handler that builds an invalid token
	// passes the test and fails against MySQL.
	if err := token.Validate(); err != nil {
		return nil, err
	}
	token.ID = int64(len(f.added) + 1)
	f.added = append(f.added, token)
	f.byBody[token.Body] = token
	return token, nil
}

func (f *fakeTokens) Edit(token *entities.Token) (*entities.Token, error) {
	if err := token.Validate(); err != nil {
		return nil, err
	}
	f.edited = append(f.edited, token)
	return token, nil
}

// ---------------------------------------------------------------------
// Auth logs and refresh tokens — between them these are "active sessions"
// ---------------------------------------------------------------------

type fakeAuthLogs struct {
	repositories.AuthLogsInterface

	logs []*entities.AuthLog
}

func (f *fakeAuthLogs) Browse(user *entities.User) ([]*entities.AuthLog, error) {
	return f.logs, nil
}

type fakeRefreshTokens struct {
	repositories.OAuthRefreshTokensInterface

	byID map[int64]*entities.OAuthRefreshToken

	revoked []int64
}

func (f *fakeRefreshTokens) ReadByID(id int64) (*entities.OAuthRefreshToken, error) {
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("no refresh token %d", id)
}

func (f *fakeRefreshTokens) Revoke(id int64) error {
	f.revoked = append(f.revoked, id)
	return nil
}

// ---------------------------------------------------------------------
// Passkeys
// ---------------------------------------------------------------------

type fakeWebAuthnCredentials struct {
	repositories.WebAuthnCredentialsInterface

	count      int64
	deletedFor []int64
	err        error
}

func (f *fakeWebAuthnCredentials) DeleteByPersonID(personID int64) (int64, error) {
	f.deletedFor = append(f.deletedFor, personID)
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

// ---------------------------------------------------------------------
// Session store
// ---------------------------------------------------------------------

// fakeSession is an in-memory stand-in for Redis, implementing the whole
// of SessionInterface because the rate limiter and the IdP-session purge
// each reach for a different corner of it.
//
// incrErr forces the failure path: the limiter is documented as failing
// OPEN, and that is a decision worth a test rather than a comment.
type fakeSession struct {
	mu sync.Mutex

	counters map[string]int64
	markers  map[string]string
	sets     map[string][]string
	deleted  []string

	incrErr   error
	existsErr error
}

func newFakeSession() *fakeSession {
	return &fakeSession{
		counters: map[string]int64{},
		markers:  map[string]string{},
		sets:     map[string][]string{},
	}
}

func (f *fakeSession) Incr(key string, ttl time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.incrErr != nil {
		return 0, f.incrErr
	}
	f.counters[key]++
	return f.counters[key], nil
}

func (f *fakeSession) Del(keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.counters, k)
		delete(f.markers, k)
		delete(f.sets, k)
		f.deleted = append(f.deleted, k)
	}
	return nil
}

func (f *fakeSession) SAdd(key string, member string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets[key] = append(f.sets[key], member)
	return nil
}

func (f *fakeSession) SMembers(key string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sets[key], nil
}

func (f *fakeSession) Exists(key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.markers[key]
	return ok, nil
}

func (f *fakeSession) SetNX(key, value string, ttl time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.markers[key]; ok {
		return false, nil
	}
	f.markers[key] = value
	return true, nil
}

// errStoreDown stands in for Redis being unavailable. Two guards degrade in
// opposite directions on it — the rate limiter opens, the step-up gate
// closes — so it is shared rather than re-declared per test.
var errStoreDown = errors.New("session store unavailable")

func (f *fakeSession) Add(echo.Context, string, string) (string, error) { return "", nil }
func (f *fakeSession) AddWithTTL(echo.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (f *fakeSession) Read(echo.Context, string) (string, error) { return "", nil }
func (f *fakeSession) Edit(echo.Context, string, string) error   { return nil }
func (f *fakeSession) Delete(echo.Context, string) error         { return nil }
func (f *fakeSession) HasSessionByID(echo.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeSession) DeleteBySessionID(echo.Context, string, string) error { return nil }
func (f *fakeSession) GetViewerSessionID(echo.Context, string) (string, error) {
	return "", nil
}

var _ sessions.SessionInterface = (*fakeSession)(nil)

// ---------------------------------------------------------------------
// Mail
// ---------------------------------------------------------------------

// fakeMail captures what would have been sent.
//
// Needed rather than optional: the notifiers are fired from goroutines
// that dereference Srv.MailGRPC, so a nil one panics on a background
// goroutine and takes the whole test binary with it.
type fakeMail struct {
	mu sync.Mutex

	sent []sentMail
}

type sentMail struct {
	To       []string
	Subject  string
	Template string
	Data     any
}

func (f *fakeMail) SendFromTemplate(from string, to, cc, bcc []string, subject, template string, data any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{To: to, Subject: subject, Template: template, Data: data})
	return nil
}

func (f *fakeMail) Send(from string, to, cc, bcc []string, subject, message string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentMail{To: to, Subject: subject})
	return "ok", nil
}

// messages returns a copy, so an assertion cannot race the fire-and-forget
// goroutine that is still writing.
func (f *fakeMail) messages() []sentMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]sentMail, len(f.sent))
	copy(out, f.sent)
	return out
}

var _ libservices.MailGRPCInterface = (*fakeMail)(nil)

// ---------------------------------------------------------------------
// Env
// ---------------------------------------------------------------------

type testEnv struct {
	*Env

	Users    *fakeUsers
	Tokens   *fakeTokens
	AuthLogs *fakeAuthLogs
	Refresh  *fakeRefreshTokens
	Passkeys *fakeWebAuthnCredentials
	Sess     *fakeSession
	Mail     *fakeMail
}

// newTestEnv wires an Env out of doubles.
//
// Env is what every handler embeds, so building one honestly — through
// notifiers.NewNotifiers rather than around it — is what makes these
// tests exercise the notifier wiring too. That wiring is where the
// long-standing mail bugs lived.
func newTestEnv(users ...*entities.User) *testEnv {
	cfg := &config.Config{
		Env:         "dev",
		DomainName:  "example.com",
		SystemEmail: "system@example.com",
	}

	te := &testEnv{
		Users:    newFakeUsers(users...),
		Tokens:   newFakeTokens(),
		AuthLogs: &fakeAuthLogs{},
		Refresh:  &fakeRefreshTokens{byID: map[int64]*entities.OAuthRefreshToken{}},
		Passkeys: &fakeWebAuthnCredentials{},
		Sess:     newFakeSession(),
		Mail:     &fakeMail{},
	}

	repos := &repositories.Repositories{
		Users:               te.Users,
		Tokens:              te.Tokens,
		AuthLogs:            te.AuthLogs,
		OAuthRefreshTokens:  te.Refresh,
		WebAuthnCredentials: te.Passkeys,
	}

	srv := &services.Services{MailGRPC: te.Mail}

	te.Env = &Env{
		Config:  cfg,
		Repos:   repos,
		Session: te.Sess,
		Srv:     srv,
		Notif: notifiers.NewNotifiers(&notifiers.Env{
			Config: cfg,
			Repos:  repos,
			Srv:    srv,
		}),
	}

	return te
}

// ---------------------------------------------------------------------
// graph-grpc, as far as auth-service uses it: who administers a group, and
// what that group is called.

type fakeActorAdmins struct {
	libservices.ActorAdminsGRPCInterface

	byActor map[int64][]*lib_entities.AnahitaNode
	err     error
}

func (f *fakeActorAdmins) Browse(actorID int64) ([]*lib_entities.AnahitaNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byActor[actorID], nil
}

type fakeActors struct {
	libservices.ActorsGRPCInterface

	byID map[int64]*lib_entities.AnahitaNode
	err  error
}

func (f *fakeActors) Read(id int64, viewer *lib_entities.Viewer) (*lib_entities.AnahitaNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	if node, ok := f.byID[id]; ok {
		return node, nil
	}
	return nil, fmt.Errorf("no actor %d", id)
}

// newActorNode builds the minimum a lifecycle notification needs.
func newActorNode(id int64, name, alias string) *lib_entities.AnahitaNode {
	node := lib_entities.NewAnahitaNode()
	node.ID = id
	node.Name = name
	node.Alias = alias
	return node
}

// waitFor blocks until n messages have been sent, or fails.
//
// The notifications go out on goroutines — the handler must not block a broker
// message on SMTP — so a test that reads the slice immediately races them.
func (f *fakeMail) waitFor(t *testing.T, n int) []sentMail {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := f.snapshot(); len(got) >= n {
			return got
		}
		time.Sleep(5 * time.Millisecond)
	}

	got := f.snapshot()
	t.Fatalf("waited for %d messages, saw %d", n, len(got))
	return got
}

func (f *fakeMail) snapshot() []sentMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMail(nil), f.sent...)
}

// expectNoneMatching asserts that no message whose subject contains substr
// arrives.
//
// A negative assertion has to WAIT. The notifications go out on goroutines, so
// reading the slice immediately passes whether the code is right or wrong —
// which is exactly how three of these tests survived a mutation that made them
// send the wrong mail entirely.
//
// The settle time is the cost of the assertion being real. It is short because
// the fake mailer does no I/O; the goroutine only has to be scheduled.
func (f *fakeMail) expectNoneMatching(t *testing.T, substr string) {
	t.Helper()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, m := range f.snapshot() {
			if substr == "" || strings.Contains(m.Subject, substr) {
				t.Fatalf("a message was sent that should not have been: %q",
					m.Subject)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}
