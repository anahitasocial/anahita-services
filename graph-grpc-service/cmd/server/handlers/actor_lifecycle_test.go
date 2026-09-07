package handlers

import (
	"testing"

	"anahita-libs/event"
)

// Deleting a person published an event and deleting a group published nothing,
// because each service emitted its own — and group-service never did. Both now
// come from here, where the state changes.
//
// The routing key is the part worth testing: a wrong one publishes to a topic
// nobody listens on, which is indistinguishable from success at this end and
// from silence at the other.

func TestActorLifecycleRoutingKey(t *testing.T) {
	cases := []struct {
		name    string
		format  string
		typ     string
		service string
		want    string
	}{
		{
			// The key person-service used to hardcode. It must not change:
			// consumers are bound to it.
			name:    "person deleted matches what person-service published",
			format:  event.EventFormatNodeActorDeleted,
			typ:     "node.actor.person-service.person.v1",
			service: "person-service",
			want:    "node.actor.person.person.deleted.v1",
		},
		{
			// The one that produced nothing at all before.
			name:    "group deleted",
			format:  event.EventFormatNodeActorDeleted,
			typ:     "node.actor.group-service.group.v1",
			service: "group-service",
			want:    "node.actor.group.group.deleted.v1",
		},
		{
			name:    "group restored",
			format:  event.EventFormatNodeActorRestored,
			typ:     "node.actor.group-service.group.v1",
			service: "group-service",
			want:    "node.actor.group.group.restored.v1",
		},
		{
			name:    "person restored",
			format:  event.EventFormatNodeActorRestored,
			typ:     "node.actor.person-service.person.v1",
			service: "person-service",
			want:    "node.actor.person.person.restored.v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := actorLifecycleRoutingKey(tc.format, tc.typ, tc.service)
			if !ok {
				t.Fatalf("refused a well-formed actor type %q", tc.typ)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q — consumers bind to this string",
					got, tc.want)
			}
		})
	}
}

// Refusing is the right answer for input that is not an actor type. A
// malformed key is worse than no key: no event reads as an absence, a wrong
// one reads as a delivery.
func TestActorLifecycleRoutingKeyRefusesMalformedInput(t *testing.T) {
	cases := []struct{ name, typ, service string }{
		{"empty type", "", "person-service"},
		{"empty service", "node.actor.person-service.person.v1", ""},
		{"too few segments", "node.actor.person", "person-service"},
		{"empty entity segment", "node.actor.person-service..v1", "person-service"},
		{"not a node type at all", "nonsense", "person-service"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := actorLifecycleRoutingKey(
				event.EventFormatNodeActorDeleted, tc.typ, tc.service)
			if ok {
				t.Errorf("accepted %q/%q and produced %q",
					tc.typ, tc.service, got)
			}
		})
	}
}

// A medium type has the same shape, and the guard must not accidentally accept
// one: publishing a medium under an actor routing key would deliver a photo to
// consumers expecting an account.
func TestActorLifecycleRoutingKeyIsNotUsedForMedia(t *testing.T) {
	key, ok := actorLifecycleRoutingKey(
		event.EventFormatNodeActorDeleted,
		"node.medium.photo-service.photo.v1",
		"photo-service",
	)

	// It DOES produce a key — the shapes are identical, so the function cannot
	// tell them apart, and pretending otherwise would be a false guarantee.
	// What protects this is the call site: publishActorLifecycle is only
	// reached from the actor handler. This test records that, so anyone
	// tempted to reuse the helper elsewhere sees the constraint.
	if !ok {
		t.Skip("shape check rejected it; the call-site constraint is then moot")
	}
	if key != "node.actor.photo.photo.deleted.v1" {
		t.Errorf("got %q", key)
	}
	t.Log("the helper cannot distinguish actor from medium types; only the " +
		"actor handler may call it")
}
