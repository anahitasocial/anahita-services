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
// provider swappable, and it keeps a provider free of per-message state: one
// instance is shared across every gRPC call, so a template name or data kept
// on it would let concurrent sends race and one send inherit another's.
//
// The context is here so a hung server cannot pin the RPC goroutine forever,
// which is what the old stdlib smtp.SendMail path did.
type MailProviderInterface interface {
	Send(ctx context.Context, msg Message) (string, error)
}

// NewProvider initializes the mail provider.
//
// SMTP is the only one: every mail provider offers a relay, so none is lost
// as a supplier, and no vendor's API, credentials or hosted templates are
// needed. The messages' templates are in this repository.
func NewProvider(config *config.Config) (MailProviderInterface, error) {
	return NewSmtpMailer(config), nil
}
