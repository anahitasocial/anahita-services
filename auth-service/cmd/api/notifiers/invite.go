package notifiers

import (
	"errors"
	"fmt"
	"log"

	"auth/constants"
	"auth/data/entities"
	"auth/messages"
	"auth/requests"

	an_constants "anahita-libs/constants"
	an_event "anahita-libs/event"
)

type inviteNotifier struct {
	*Env
}

func newInviteNotifier(env *Env) *inviteNotifier {
	return &inviteNotifier{env}
}

// EmailInvite sends the invitation email with a link to the server-
// rendered accept form on the auth-service host (NOT the BFF host).
// The legacy /auth/register/{token} SPA route is being retired.
func (h *inviteNotifier) EmailInvite(user *entities.User, recipientEmail string, token *entities.Token) error {
	body := fmt.Sprintf(
		messages.InviteBody,
		user.GivenName,
		user.FamilyName,
	)

	// Point at the auth-service form. The token rides as a query
	// param; the handler re-validates it on the GET and again on the
	// POST.
	actionURL := fmt.Sprintf(
		"%s/invites/accept?token=%s",
		h.Config.GetAuthURL(),
		token.Body,
	)

	data := map[string]string{
		"body":        body,
		"actionLabel": messages.InviteActionLabel,
		"actionURL":   actionURL,
		"title":       messages.InviteTitle,
	}

	return h.Srv.MailGRPC.SendFromTemplate(
		h.Config.SystemEmail,
		[]string{recipientEmail},
		[]string{},
		[]string{},
		messages.InviteSubject,
		"message_action",
		data,
	)
}

// NotifyAccept publishes the in-app event for an invite acceptance.
// The notification-service handles fan-out to whichever recipients
// it's configured to broadcast member-change events to.
//
// Kept separate from EmailAccept so the email path and the in-app
// path can fail independently — a transient SMTP failure shouldn't
// suppress the in-app notification, and vice versa.
func (h *inviteNotifier) NotifyAccept(user *entities.User) error {
	payload := &an_event.EventPayload{
		Name:          an_event.EventFormatEntityUserSignup,
		SourceService: constants.ConfigServiceName,
		SubjectID:     user.ID,
	}

	log.Println("Publishing event", payload.Name)
	if err := h.Publisher.Publish(payload, payload.Name); err != nil {
		return err
	}

	return nil
}

// EmailAccept broadcasts the "new member joined" email to every admin
// and super-admin on the site. The body is picked per
// recipient: the inviter gets a self-addressed variant ("your
// invitation"), while other admins get a variant that names the
// inviter.
//
// Errors from individual sends are accumulated and joined rather
// than short-circuiting the loop — one failed SMTP delivery should
// not silently skip the remaining admins.
func (h *inviteNotifier) EmailAccept(joiner *entities.User, inviter *entities.User) error {
	personReq := requests.NewUserRequest().Browse
	personReq.UserTypes = []string{
		an_constants.UserTypeAdmin,
		an_constants.UserTypeSuperAdmin,
	}

	admins, err := h.Repos.Users.Browse(personReq)
	if err != nil {
		return err
	}

	actionURL := fmt.Sprintf(
		"%s/people/%d",
		h.Config.GetCompleteDomainName(),
		joiner.ID,
	)

	var sendErrors []error
	for _, admin := range admins {
		// Skip the joiner if they happen to appear in the recipient
		// set. In practice the joiner is PersonTypeRegistered so
		// they won't be here, but cheap insurance against future
		// changes that might invite people directly into admin roles.
		if admin.ID == joiner.ID {
			continue
		}

		// Two body variants: one for the inviter (self-addressed),
		// one for the inviter (self-addressed),
		// one for everyone else. ID == 0 on the inviter means
		// the inviter lookup failed in the handler — fall back to
		// the generic body for everyone in that case.
		var body string
		if inviter.ID != 0 && admin.ID == inviter.ID {
			body = fmt.Sprintf(
				messages.InviteAcceptBodyToInviter,
				admin.GivenName,
				joiner.GivenName, joiner.FamilyName,
			)
		} else {
			body = fmt.Sprintf(
				messages.InviteAcceptBody,
				admin.GivenName,
				joiner.GivenName, joiner.FamilyName,
				inviter.GivenName, inviter.FamilyName,
			)
		}

		data := map[string]string{
			"body":        body,
			"actionLabel": messages.InviteAcceptActionLabel,
			"actionURL":   actionURL,
			"title":       messages.InviteAcceptTitle,
		}

		if err := h.Srv.MailGRPC.SendFromTemplate(
			h.Config.SystemEmail,
			[]string{admin.Email},
			[]string{},
			[]string{},
			messages.InviteAcceptSubject,
			"message_action",
			data,
		); err != nil {
			sendErrors = append(sendErrors,
				fmt.Errorf("invite accept email to admin %d failed: %w", admin.ID, err))
		}
	}

	return errors.Join(sendErrors...)
}
