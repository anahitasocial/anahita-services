package handlers

import (
	"anahita-libs/proto/mailpb"
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"mail/config"
	"mail/providers"
)

// recordingProvider captures what the handler actually asked to be sent, which
// is the only way to tell a redirect that worked from one that quietly did
// nothing — both return success.
type recordingProvider struct {
	msg    providers.Message
	called bool
}

func (p *recordingProvider) Send(ctx context.Context, msg providers.Message) (string, error) {
	p.msg = msg
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
		TemplateData: `{"title":"Confirm","body":"hello","actionLabel":"Confirm","actionURL":"https://example.com/go"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.msg.To) != 1 || provider.msg.To[0] != "developer@example.com" {
		t.Errorf("expected the developer as the only recipient, got %v", provider.msg.To)
	}

	// Not redirected as well — that would deliver the same message three times.
	if len(provider.msg.Cc) != 0 || len(provider.msg.Bcc) != 0 {
		t.Errorf("expected cc and bcc to be dropped, got cc=%v bcc=%v",
			provider.msg.Cc, provider.msg.Bcc)
	}

	// The point of the prefix: an inbox full of redirected mail is unreadable
	// unless each message says who it was really for.
	for _, want := range []string{"alice@example.com", "bob@example.com", "carol@example.com"} {
		if !strings.Contains(provider.msg.Subject, want) {
			t.Errorf("expected subject to name %s, got %q", want, provider.msg.Subject)
		}
	}
	if !strings.Contains(provider.msg.Subject, "Confirm your new email address") {
		t.Errorf("expected the original subject to survive, got %q", provider.msg.Subject)
	}

	// The redirect must not disturb what the message actually is.
	if !strings.Contains(provider.msg.Text, "hello") {
		t.Errorf("body missing from the text part: %q", provider.msg.Text)
	}
	if !strings.Contains(provider.msg.HTML, "https://example.com/go") {
		t.Error("action URL missing from the HTML part")
	}
	if provider.msg.From != "system@example.com" {
		t.Errorf("sender altered: %q", provider.msg.From)
	}
}

func TestSendFromTemplate_NoRedirectOutsideDev(t *testing.T) {
	for _, env := range []string{"staging", "prod"} {
		t.Run(env, func(t *testing.T) {
			h, provider := newHandler(env, "developer@example.com")

			_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
				From:         "system@example.com",
				To:           []string{"alice@example.com"},
				Cc:           []string{"bob@example.com"},
				Subject:      "Welcome",
				Template:     "message_action",
				TemplateData: `{"title":"Welcome","body":"hello"}`,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(provider.msg.To) != 1 || provider.msg.To[0] != "alice@example.com" {
				t.Errorf("expected the real recipient, got %v", provider.msg.To)
			}
			// Carried through rather than dropped, which is what the provider
			// interface could not express before.
			if len(provider.msg.Cc) != 1 || provider.msg.Cc[0] != "bob@example.com" {
				t.Errorf("expected cc to survive, got %v", provider.msg.Cc)
			}
			if provider.msg.Subject != "Welcome" {
				t.Errorf("expected an unprefixed subject, got %q", provider.msg.Subject)
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
	if len(provider.msg.To) != 1 || provider.msg.To[0] != "developer@example.com" {
		t.Errorf("expected one message to the developer, got %v", provider.msg.To)
	}
	if !strings.Contains(provider.msg.Subject, "alice@example.com") ||
		!strings.Contains(provider.msg.Subject, "dave@example.com") {
		t.Errorf("expected both original recipients in the subject, got %q", provider.msg.Subject)
	}
}

// An unknown template name is now an error rather than something forwarded to a
// provider to fail on later — or, worse, to accept and silently drop. Every
// notifier is fire-and-forget, so a failure that does not happen here does not
// reach anybody.
func TestSendFromTemplate_RejectsUnknownTemplate(t *testing.T) {
	h, provider := newHandler("prod", "")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:     "system@example.com",
		To:       []string{"alice@example.com"},
		Subject:  "Hello",
		Template: "no_such_template",
	})
	if err == nil {
		t.Fatal("expected an error for an unknown template")
	}
	if provider.called {
		t.Error("the provider was called with an unrenderable message")
	}
}

// Malformed template data must not reach the provider either.
func TestSendFromTemplate_RejectsMalformedData(t *testing.T) {
	h, provider := newHandler("prod", "")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:         "system@example.com",
		To:           []string{"alice@example.com"},
		Subject:      "Hello",
		Template:     "message_action",
		TemplateData: `{"body":`,
	})
	if err == nil {
		t.Fatal("expected an error for malformed template data")
	}
	if provider.called {
		t.Error("the provider was called despite unparseable data")
	}
}

// Testing against your own address makes DEVELOPER_EMAIL the real recipient
// too. The redirect then moves nothing, and prefixing the subject is noise on
// a message that went exactly where it was addressed.
func TestDevRedirectDoesNotPrefixWhenItChangesNothing(t *testing.T) {
	h, provider := newHandler("dev", "rastin@anahita.io")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:         "Anahita <system@anahita.io>",
		To:           []string{"rastin@anahita.io"},
		Subject:      "Reset your Anahita password",
		Template:     "message_action",
		TemplateData: `{"title":"Reset your password","body":"hello"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider.msg.Subject != "Reset your Anahita password" {
		t.Errorf("subject = %q, want it unprefixed", provider.msg.Subject)
	}
}

// Case and surrounding space must not defeat the comparison — a redirect that
// moved nothing should not be reported as one that did.
func TestDevRedirectNoOpIsCaseInsensitive(t *testing.T) {
	h, provider := newHandler("dev", "Rastin@Anahita.io")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:         "Anahita <system@anahita.io>",
		To:           []string{"rastin@anahita.io"},
		Subject:      "Reset your password",
		Template:     "message_action",
		TemplateData: `{"title":"t","body":"b"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(provider.msg.Subject, "[dev") {
		t.Errorf("subject = %q, want it unprefixed", provider.msg.Subject)
	}
}

// But a genuine redirect must still say so, or an inbox of forwarded mail has
// no way to show who each message was for.
func TestDevRedirectStillPrefixesWhenItMovesTheMessage(t *testing.T) {
	h, provider := newHandler("dev", "developer@example.com")

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:         "Anahita <system@anahita.io>",
		To:           []string{"someone-else@example.com"},
		Subject:      "Reset your password",
		Template:     "message_action",
		TemplateData: `{"title":"t","body":"b"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(provider.msg.Subject, "someone-else@example.com") {
		t.Errorf("subject = %q, want it to name the original recipient", provider.msg.Subject)
	}
}

// A token in a log has effectively escaped: logs are shipped, tailed and
// pasted into chat windows. The request log must carry enough to trace a
// message and nothing that could send one.
func TestRequestLogOmitsTheTemplatePayload(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	h, _ := newHandler("prod", "")

	const token = "e755a3c19ee171976c38c30592b9362fd863de4ea0a9b4dab46070b64e0e553f"

	_, err := h.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
		From:     "Anahita <system@anahita.io>",
		To:       []string{"rastin@anahita.io"},
		Subject:  "Reset your Anahita password",
		Template: "message_action",
		TemplateData: `{"title":"Reset your password","body":"hello",` +
			`"actionLabel":"Reset password",` +
			`"actionURL":"http://localhost/password/reset?token=` + token + `"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logged := buf.String()

	if strings.Contains(logged, token) {
		t.Error("the reset token was written to the log; anyone reading it can take the account")
	}
	if strings.Contains(logged, "template_data") || strings.Contains(logged, "actionURL") {
		t.Errorf("the template payload reached the log:\n%s", logged)
	}

	// Still traceable: recipient, subject and template name are all there.
	for _, want := range []string{"rastin@anahita.io", "message_action", "Reset your Anahita password"} {
		if !strings.Contains(logged, want) {
			t.Errorf("the log does not mention %q, so a send cannot be traced", want)
		}
	}
}
