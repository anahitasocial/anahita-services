package handlers

import (
	"anahita-libs/proto/mailpb"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type mailHandler struct {
	mailpb.MailServiceServer
	env *Env
}

func NewMailHandler(env *Env) *mailHandler {
	return &mailHandler{env: env}
}

// logRequest records that a send was asked for, WITHOUT the payload.
//
// The previous form was log.Printf("...%v", req), which prints the whole
// protobuf — including template_data. That map carries the password-reset
// token, the email-change token and the invite token, each of which is a live
// credential: whoever holds the reset token can take the account without ever
// seeing the mailbox. Logs get shipped, tailed and pasted into chat windows,
// so a token in one has effectively escaped.
//
// Recipients, subject and template name are enough to trace a message. The
// data that makes it work is not.
func logRequest(rpc, from string, to, cc, bcc []string, subject, template string) {
	recipients := len(to) + len(cc) + len(bcc)

	if template != "" {
		log.Printf("mail: %s from=%s to=%s recipients=%d template=%q subject=%q",
			rpc, from, strings.Join(to, ", "), recipients, template, subject)
		return
	}

	log.Printf("mail: %s from=%s to=%s recipients=%d subject=%q",
		rpc, from, strings.Join(to, ", "), recipients, subject)
}

// devRedirect sends everything to the developer instead of the real recipients.
//
// Development only. Every flow worth testing in auth-service mails somebody —
// password reset, email change, account deletion — and several of them cannot
// be completed without receiving the message, because the link inside it is the
// next step. Without this, exercising them means either mailing real people
// from a laptop or not exercising them.
//
// Returns the recipients to send to and the subject to send, so the caller
// cannot apply one without the other.
//
// THE ORIGINAL RECIPIENTS GO INTO THE SUBJECT. protect-services drops them, and
// the result is an inbox full of identical-looking messages with no way to tell
// which one was meant for whom — which matters most for the flows that mail
// several different people, like the invite notifier that writes to every
// administrator in turn. A subject prefix survives both RPCs and needs no
// provider-specific header support.
//
// Note this COLLAPSES rather than fans out: a message addressed to five people
// becomes one message, not five.
func (h *mailHandler) devRedirect(to, cc, bcc []string, subject string) ([]string, []string, []string, string) {
	if !h.env.Config.IsDev() {
		return to, cc, bcc, subject
	}

	original := make([]string, 0, len(to)+len(cc)+len(bcc))
	original = append(original, to...)
	original = append(original, cc...)
	original = append(original, bcc...)

	developer := h.env.Config.GetDeveloperEmail()

	log.Printf("mail: development mode, redirecting to %s (originally %s)",
		developer, strings.Join(original, ", "))

	// Prefix only when the redirect actually moved something. Testing against
	// your own address makes DEVELOPER_EMAIL the real recipient too, and
	// "[dev → you@example.com]" in front of every subject is then pure noise
	// on top of a message that went exactly where it was addressed.
	if len(original) > 0 && !onlyRecipientIs(original, developer) {
		subject = fmt.Sprintf("[dev → %s] %s", strings.Join(original, ", "), subject)
	}

	// Cc and Bcc are emptied, not redirected. Leaving them would deliver the
	// same message to the developer several times over.
	return []string{developer}, nil, nil, subject
}

// onlyRecipientIs reports whether a message was already addressed solely to
// the given address, making the redirect a no-op.
func onlyRecipientIs(recipients []string, address string) bool {
	if len(recipients) != 1 {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(recipients[0]), strings.TrimSpace(address))
}

// Send send an email
func (h *mailHandler) Send(ctx context.Context, req *mailpb.SendRequest) (*mailpb.SendResponse, error) {
	logRequest("Send", req.From, req.To, req.Cc, req.Bcc, req.Subject, "")

	to, cc, bcc, subject := h.devRedirect(req.To, req.Cc, req.Bcc, req.Subject)

	// A pre-composed body, sent as-is. is_html says which half of the message
	// the caller supplied; the other half is left empty rather than
	// manufactured, because inventing a plain-text rendering of somebody
	// else's HTML is guesswork.
	msg := toMessage(req.From, to, cc, bcc, subject, req.Message, "")
	if req.IsHtml {
		msg.HTML = req.Message
		// The text part is required, and a bare HTML document is a poor one,
		// but it beats no text part at all for deliverability.
		msg.Text = req.Message
	}

	response, err := h.env.Provider.Send(ctx, msg)
	if err != nil {
		log.Printf("mail: Send FAILED to %s: %v", strings.Join(to, ", "), err)
		return nil, err
	}

	if response == "" {
		log.Printf("mail: Send to %s reported no status", strings.Join(to, ", "))
		return nil, fmt.Errorf("failed to send email")
	}

	log.Printf("mail: Send delivered to %s", strings.Join(to, ", "))

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}

// SendFromTemplate send an email using a predefined template
func (h *mailHandler) SendFromTemplate(ctx context.Context, req *mailpb.SendFromTemplateRequest) (*mailpb.SendResponse, error) {
	logRequest("SendFromTemplate", req.From, req.To, req.Cc, req.Bcc, req.Subject, req.Template)

	// Rendered HERE, from a template in this repository, rather than forwarded
	// to a provider to resolve against one of theirs. That move is the whole
	// point of the change: the appearance of every email the system sends is
	// now in version control and covered by tests.
	var raw map[string]string
	if req.TemplateData != "" {
		if err := json.Unmarshal([]byte(req.TemplateData), &raw); err != nil {
			return nil, fmt.Errorf("template data: %w", err)
		}
	}

	text, html, err := renderTemplate(req.Template, h.templateData(raw))
	if err != nil {
		return nil, err
	}

	// AFTER rendering, so a template failure is reported before anything is
	// addressed, and BEFORE sending, so development mail cannot escape.
	to, cc, bcc, subject := h.devRedirect(req.To, req.Cc, req.Bcc, req.Subject)

	response, err := h.env.Provider.Send(ctx, toMessage(req.From, to, cc, bcc, subject, text, html))
	if err != nil {
		// Logged HERE as well as returned. The caller is a fire-and-forget
		// goroutine in another service, so a returned error reaches a log line
		// nobody is watching; this service's log is where somebody debugging a
		// missing email actually looks.
		log.Printf("mail: SendFromTemplate %q FAILED to %s: %v",
			req.Template, strings.Join(to, ", "), err)
		return nil, err
	}

	if response == "" {
		log.Printf("mail: SendFromTemplate %q to %s reported no status",
			req.Template, strings.Join(to, ", "))
		return nil, fmt.Errorf("failed to send email")
	}

	log.Printf("mail: SendFromTemplate %q delivered to %s",
		req.Template, strings.Join(to, ", "))

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}
