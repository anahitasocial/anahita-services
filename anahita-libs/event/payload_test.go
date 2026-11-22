package event

import "testing"

// What an event says was done is the word before its version, whatever
// comes before it, and nothing for a name that is not one.
func TestAnEventsActionIsReadFromItsName(t *testing.T) {
	for name, want := range map[string]string{
		"node.medium.text-service.note.added.v1":       ActionAdded,
		"edge.follow.person-service.deleted.v1":        ActionDeleted,
		"node.actor.person-service.person.purged.v1":   ActionPurged,
		"federation.medium.text-service.note.added.v1": ActionAdded,
		"edge.tag.photo-service.requested.v1":          ActionRequested,
		"node.actor.actor-service.event.answered.v2":   ActionAnswered,
		"":                                    "",
		"added":                               "",
		"node.medium.text-service.note.added": "",
	} {
		if got := (&EventPayload{Name: name}).Action(); got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
}
