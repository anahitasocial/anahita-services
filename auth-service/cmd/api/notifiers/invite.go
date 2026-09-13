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

	return h.send(
		recipientEmail,
		messages.InviteSubject,
		messages.InviteTitle,
		body,
		messages.InviteActionLabel,
		actionURL,
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
	personReq.PersonTypes = []string{
		an_constants.PersonTypeAdmin,
		an_constants.PersonTypeSuperAdmin,
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

		if err := h.send(
			admin.Email,
			messages.InviteAcceptSubject,
			messages.InviteAcceptTitle,
			body,
			messages.InviteAcceptActionLabel,
			actionURL,
		); err != nil {
			// Accumulated rather than returned: one administrator with a
			// dead address must not stop the others being told.
			sendErrors = append(sendErrors,
				fmt.Errorf("invite accept email to admin %d failed: %w", admin.ID, err))
		}
	}

	return errors.Join(sendErrors...)
}

// EmailRevoked tells somebody their pending invitation was withdrawn.
//
// Sent to an address with no account behind it, which is why there is no
// action link: the recipient has nothing to sign in to and nothing to secure.
// The alternative was silence — a link that simply stops working — which reads
// as a broken site rather than a decision.
func (h *inviteNotifier) EmailRevoked(revoker *entities.User, recipientEmail string) error {
	body := fmt.Sprintf(
		messages.InviteRevokedBody,
		revoker.GivenName,
		revoker.FamilyName,
	)

	return h.send(
		recipientEmail,
		messages.InviteRevokedSubject,
		messages.InviteRevokedTitle,
		body,
		messages.InviteRevokedActionLabel,
		h.Config.GetFrontendURL(),
	)
}
