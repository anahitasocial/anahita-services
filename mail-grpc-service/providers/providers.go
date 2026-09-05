package providers

import (
	"context"

	"mail/config"
)

// Message is one outbound email, fully rendered.
//
// Both bodies are present on every message: Text is what a plain-text client
// and most spam filters read, HTML is what everyone else sees. A provider
// assembles them into multipart/alternative rather than choosing between them.
type Message struct {
	From    string
	To      []string
	Cc      []string
	Bcc     []string
	Subject string

	// Text is required; HTML is optional. A message with no text part is a
	// deliverability penalty for no benefit, so the renderer always produces
	// both and the provider treats a missing Text as a programming error.
	Text string
	HTML string
}

// MailProviderInterface is the interface that all mail providers must
// implement.
//
// ONE method, and it takes a rendered message. Templates are deliberately
// absent: rendering belongs to the service, which owns the templates, not to
// whatever happens to be carrying the bytes. That split is what makes the
// provider swappable, and it removed a real bug along with the old shape — the
// Mailgun and SES implementations stored the template name and data as fields
// on a single provider instance shared across every gRPC call, so concurrent
// sends raced and a Send following a SendFromTemplate silently inherited the
// previous call's template.
//
// The context is here so a hung server cannot pin the RPC goroutine forever,
// which is what the old stdlib smtp.SendMail path did.
type MailProviderInterface interface {
	Send(ctx context.Context, msg Message) (string, error)
}

// NewProvider initializes the mail provider.
//
// SMTP is the only one. The Mailgun and AWS SES API clients were removed along
// with the MAILER_PROVIDER switch: both vendors expose SMTP, so neither is lost
// as a supplier, and the API paths cost vendor-specific credentials plus — in
// Mailgun's case — a message template living in the vendor's dashboard instead
// of in this repository.
func NewProvider(config *config.Config) (MailProviderInterface, error) {
	return NewSmtpMailer(config), nil
}
