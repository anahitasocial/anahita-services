package handlers

import (
	"anahita-libs/proto/mailpb"
	"context"
	"strings"
	"testing"

	"mail/config"
)

// recordingProvider captures what the handler actually asked to be sent, which
// is the only way to tell a redirect that worked from one that quietly did
// nothing — both return success.
type recordingProvider struct {
	from     string
	to       []string
	cc       []string
	bcc      []string
	subject  string
	message  string
	template string
	data     string
	called   bool
}

func (p *recordingProvider) Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error) {
	p.from, p.to, p.cc, p.bcc, p.subject, p.message = from, to, cc, bcc, subject, message
	p.called = true
	return "sent", nil
}

func (p *recordingProvider) SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data string) (string, error) {
	p.from, p.to, p.cc, p.bcc, p.subject = from, to, cc, bcc, subject
	p.template, p.data = template, data
	p.called = true
	return "sent", nil
}

func newHandler(env string, developer string) (*mailHandler, *recordingProvider) {
	provider := &recordingProvider{}
	return NewMailHandler(&Env{
		Config:   &config.Config{Env: env, DeveloperEmail: developer},
		Provider: provider,
	}), provider
}

func TestSendFromTemplate_RedirectsInDev(t *testing.T) {
	h, provider := newHandler("dev", "developer@example.com")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:         "system@example.com",
		To:           []string{"alice@example.com"},
		Cc:           []string{"bob@example.com"},
		Bcc:          []string{"carol@example.com"},
		Subject:      "Confirm your new email address",
		Template:     "message_action",
		TemplateData: `{"body":"hello"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.to) != 1 || provider.to[0] != "developer@example.com" {
		t.Errorf("expected the developer as the only recipient, got %v", provider.to)
	}

	// Not redirected as well — that would deliver the same message three times.
	if len(provider.cc) != 0 || len(provider.bcc) != 0 {
		t.Errorf("expected cc and bcc to be dropped, got cc=%v bcc=%v", provider.cc, provider.bcc)
	}

	// The point of the prefix: an inbox full of redirected mail is unreadable
	// unless each message says who it was really for.
	for _, want := range []string{"alice@example.com", "bob@example.com", "carol@example.com"} {
		if !strings.Contains(provider.subject, want) {
			t.Errorf("expected subject to name %s, got %q", want, provider.subject)
		}
	}
	if !strings.Contains(provider.subject, "Confirm your new email address") {
		t.Errorf("expected the original subject to survive, got %q", provider.subject)
	}

	// The redirect must not disturb what the message actually is.
	if provider.template != "message_action" || provider.data != `{"body":"hello"}` {
		t.Errorf("template or data altered: %q %q", provider.template, provider.data)
	}
	if provider.from != "system@example.com" {
		t.Errorf("sender altered: %q", provider.from)
	}
}

func TestSendFromTemplate_NoRedirectOutsideDev(t *testing.T) {
	for _, env := range []string{"staging", "prod"} {
		t.Run(env, func(t *testing.T) {
			h, provider := newHandler(env, "developer@example.com")

			_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
				From:    "system@example.com",
				To:      []string{"alice@example.com"},
				Cc:      []string{"bob@example.com"},
				Subject: "Welcome",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(provider.to) != 1 || provider.to[0] != "alice@example.com" {
				t.Errorf("expected the real recipient, got %v", provider.to)
			}
			// Carried through rather than dropped, which is what the provider
			// interface could not express before.
			if len(provider.cc) != 1 || provider.cc[0] != "bob@example.com" {
				t.Errorf("expected cc to survive, got %v", provider.cc)
			}
			if provider.subject != "Welcome" {
				t.Errorf("expected an unprefixed subject, got %q", provider.subject)
			}
		})
	}
}

func TestSend_RedirectsInDev(t *testing.T) {
	h, provider := newHandler("dev", "developer@example.com")

	_, err := h.Send(context.Background(), &mailpb.SendRequest{
		From:    "system@example.com",
		To:      []string{"alice@example.com", "dave@example.com"},
		Subject: "Hello",
		Message: "body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collapsed, not fanned out: two recipients become one message.
	if len(provider.to) != 1 || provider.to[0] != "developer@example.com" {
		t.Errorf("expected one message to the developer, got %v", provider.to)
	}
	if !strings.Contains(provider.subject, "alice@example.com") ||
		!strings.Contains(provider.subject, "dave@example.com") {
		t.Errorf("expected both original recipients in the subject, got %q", provider.subject)
	}
}
