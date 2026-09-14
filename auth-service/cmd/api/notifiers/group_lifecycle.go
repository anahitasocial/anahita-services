package notifiers

import (
	"fmt"

	"auth/data/entities"
	"auth/messages"
)

type groupLifecycleNotifier struct {
	*Env
}

func newGroupLifecycleNotifier(env *Env) *groupLifecycleNotifier {
	return &groupLifecycleNotifier{env}
}

// EmailGroupDeleted tells one administrator that a group is scheduled for
// deletion.
//
// One administrator per call rather than one call per group, because the body
// is addressed by name and the caller is already iterating the admin list. It
// also means one failed send does not cost the others their message.
//
// The action is the group's own page: restoring is done from there, and during
// the grace period the page still resolves for an administrator.
func (h *groupLifecycleNotifier) EmailGroupDeleted(admin *entities.User, groupName, groupAlias string) error {
	subject := fmt.Sprintf(messages.GroupDeletedSubject, groupName)

	body := fmt.Sprintf(
		messages.GroupDeletedBody,
		admin.Name,
		groupName,
	)

	return h.send(
		admin.Email,
		subject,
		messages.GroupDeletedTitle,
		body,
		messages.GroupDeletedActionLabel,
		h.groupURL(groupAlias),
	)
}

// EmailGroupRestored tells one administrator that a group came back.
func (h *groupLifecycleNotifier) EmailGroupRestored(admin *entities.User, groupName, groupAlias string) error {
	subject := fmt.Sprintf(messages.GroupRestoredSubject, groupName)

	body := fmt.Sprintf(
		messages.GroupRestoredBody,
		admin.Name,
		groupName,
	)

	return h.send(
		admin.Email,
		subject,
		messages.GroupRestoredTitle,
		body,
		messages.GroupRestoredActionLabel,
		h.groupURL(groupAlias),
	)
}

// groupURL builds the front-end address of a group.
//
// The alias, not the id: /groups/<alias> is the shareable form, and it is what
// the person would have in their history.
func (h *groupLifecycleNotifier) groupURL(alias string) string {
	return fmt.Sprintf("%s/groups/%s", h.Config.GetFrontendURL(), alias)
}
