package handlers

import (
	"strings"
	"testing"
	"time"

	lib_entities "anahita-libs/data/entities"
	"anahita-libs/event"
)

// Deleting a group notified nobody. A group can have several administrators
// and any one of them can delete it, so the others would find out by visiting
// a space that was no longer there.
//
// The routing key is what steers this — node.actor.group.group.deleted.v1 —
// and the fourth segment is what separates a group from a person. Getting that
// wrong sends group mail about people, or nothing at all.

// newGroupEnv wires a group with two administrators and the graph clients
// auth-service needs to reach them.
func newGroupEnv(t *testing.T) (*testEnv, *fakeMail) {
	t.Helper()

	adminOne := testUser(t, 11, "ada@example.com", "correct horse battery staple")
	adminTwo := testUser(t, 12, "grace@example.com", "correct horse battery staple")
	adminOne.Enabled, adminTwo.Enabled = true, true

	env := newTestEnv(adminOne, adminTwo)
	env.Srv.ActorAdminsGRPC = &fakeActorAdmins{
		byActor: map[int64][]*lib_entities.AnahitaNode{
			4242: {newActorNode(11, "Ada", "ada"), newActorNode(12, "Grace", "grace")},
		},
	}
	env.Srv.ActorsGRPC = &fakeActors{
		byID: map[int64]*lib_entities.AnahitaNode{
			4242: newActorNode(4242, "Cycling Club", "cycling-club"),
		},
	}

	return env, env.Mail
}

func TestGroupDeletionMailsEveryAdministrator(t *testing.T) {
	env, mail := newGroupEnv(t)
	h := newAccountDeletedHandler(env.Env)

	if err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.deleted.v1",
		ObjectID: 4242,
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}

	sent := mail.waitFor(t, 2)

	recipients := map[string]bool{}
	for _, m := range sent {
		recipients[strings.Join(m.To, ",")] = true
		if !strings.Contains(m.Subject, "Cycling Club") {
			t.Errorf("subject does not name the group: %q — \"a group you "+
				"administer was deleted\" is not actionable", m.Subject)
		}
	}

	for _, want := range []string{"ada@example.com", "grace@example.com"} {
		if !recipients[want] {
			t.Errorf("%s was not notified; any one administrator can delete a "+
				"group and the others find out by visiting it", want)
		}
	}
}

func TestGroupRestorationMailsEveryAdministrator(t *testing.T) {
	env, mail := newGroupEnv(t)
	h := newAccountDeletedHandler(env.Env)

	if err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.restored.v1",
		ObjectID: 4242,
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}

	for _, m := range mail.waitFor(t, 2) {
		if !strings.Contains(strings.ToLower(m.Subject), "restored") {
			t.Errorf("a restore was reported as %q", m.Subject)
		}
	}
}

// People take the credential path instead — auth-service already mails them
// from the deletion handler. Sending group mail about a person would be both
// wrong and a second message.
func TestPersonLifecycleDoesNotSendGroupMail(t *testing.T) {
	env, mail := newGroupEnv(t)
	h := newAccountDeletedHandler(env.Env)

	if err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.person.person.restored.v1",
		ObjectID: 4242,
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}

	mail.expectNoneMatching(t, "Cycling Club")
}

// Purge is a different branch, and must not be read as a deletion — the group
// is already gone by then and its administrators were told when it started.
func TestGroupPurgeDoesNotResendTheDeletionNotice(t *testing.T) {
	env, mail := newGroupEnv(t)
	h := newAccountDeletedHandler(env.Env)

	_ = h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.purged.v1",
		ObjectID: 4242,
	})

	mail.expectNoneMatching(t, "scheduled for deletion")
}

// A group nobody can name is not worth mailing about — the subject would carry
// an empty pair of quotes.
func TestGroupWithNoResolvableNameIsNotNotified(t *testing.T) {
	env, mail := newGroupEnv(t)
	env.Srv.ActorsGRPC = &fakeActors{byID: map[int64]*lib_entities.AnahitaNode{}}
	h := newAccountDeletedHandler(env.Env)

	if err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.deleted.v1",
		ObjectID: 4242,
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}

	// Nothing at all, not merely nothing named — the subject would carry an
	// empty pair of quotes.
	mail.expectNoneMatching(t, "")
}

// A malformed key must fall through everything rather than reaching a branch
// by accident. This handler runs off a broker message, so a panic on a short
// key would nack and redeliver forever.
func TestHandleEventsIgnoresShortRoutingKeys(t *testing.T) {
	env := newTestEnv()
	h := newAccountDeletedHandler(env.Env)

	for _, key := range []string{"", "node", "node.actor", "node.actor.group"} {
		if err := h.HandleEvents(&event.EventPayload{Name: key, ObjectID: 1}); err != nil {
			t.Errorf("HandleEvents(%q) returned %v; a malformed key must be "+
				"dropped, not nacked into a redelivery loop", key, err)
		}
	}
}

// The graph client is optional configuration. An installation without it must
// lose this one notification quietly rather than fail the message.
func TestGroupLifecycleWithoutTheGraphClientIsANoOp(t *testing.T) {
	env := newTestEnv()
	h := newAccountDeletedHandler(env.Env)

	// newTestEnv wires no ActorAdminsGRPC, which is the unconfigured case.
	err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.deleted.v1",
		ObjectID: 4242,
	})
	if err != nil {
		t.Errorf("HandleEvents returned %v; missing optional config must not "+
			"nack the message", err)
	}
}

// WHAT THIS GUARDS (roadmap row 96, part 5): the administrator who deleted
// the group knows, and is not mailed; one whose own account is disabled or
// scheduled for deletion is not mailed either.
func TestGroupDeletionMailsNeitherWhoDidItNorADisabledAdministrator(t *testing.T) {
	env, mail := newGroupEnv(t)
	h := newAccountDeletedHandler(env.Env)

	if err := h.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.deleted.v1",
		ObjectID: 4242,
		ViewerID: 11, // Ada deleted it
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}
	for _, m := range onlyOne(t, mail) {
		if strings.Join(m.To, ",") != "grace@example.com" {
			t.Errorf("mailed %v; only Grace, who did not delete it, should be", m.To)
		}
	}

	env2, mail2 := newGroupEnv(t)
	grace, _ := env2.Repos.Users.ReadByID(12)
	grace.Enabled = false
	h2 := newAccountDeletedHandler(env2.Env)
	if err := h2.HandleEvents(&event.EventPayload{
		Name:     "node.actor.group.group.deleted.v1",
		ObjectID: 4242,
	}); err != nil {
		t.Fatalf("HandleEvents: %v", err)
	}
	for _, m := range onlyOne(t, mail2) {
		if strings.Join(m.To, ",") != "ada@example.com" {
			t.Errorf("mailed %v; Grace's account is disabled", m.To)
		}
	}
}

// onlyOne waits for a message, then long enough for a second to arrive, and
// fails if one does: the mail goes out on goroutines.
func onlyOne(t *testing.T, mail *fakeMail) []sentMail {
	t.Helper()
	mail.waitFor(t, 1)
	time.Sleep(250 * time.Millisecond)
	sent := mail.snapshot()
	if len(sent) != 1 {
		t.Fatalf("%d messages sent, want 1", len(sent))
	}
	return sent
}
