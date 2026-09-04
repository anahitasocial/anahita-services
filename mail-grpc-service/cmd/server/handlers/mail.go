package handlers

import (
	"anahita-libs/proto/mailpb"
	"context"
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

	if len(original) > 0 {
		subject = fmt.Sprintf("[dev → %s] %s", strings.Join(original, ", "), subject)
	}

	// Cc and Bcc are emptied, not redirected. Leaving them would deliver the
	// same message to the developer several times over.
	return []string{developer}, nil, nil, subject
}

// Send send an email
func (h *mailHandler) Send(ctx context.Context, req *mailpb.SendRequest) (*mailpb.SendResponse, error) {
	log.Printf("Received SendEmail request: %v", req)

	var (
		response string
		err      error
		message  string
	)

	// Both templates read {{.message}}, and the HTML one also reads {{.lang}}.
	// req.Message was being passed through as a bare string, so every execution
	// failed with "can't evaluate field message in type string" — this RPC has
	// no callers yet, which is why that never surfaced.
	data := map[string]any{
		"message": req.Message,
		"lang":    "en",
	}

	if req.IsHtml {
		message, err = buildHTMLMessage(data)
		if err != nil {
			return nil, err
		}
	} else {
		message, err = buildPlainTextMessage(data)
		if err != nil {
			return nil, err
		}
	}

	to, cc, bcc, subject := h.devRedirect(req.To, req.Cc, req.Bcc, req.Subject)

	response, err = h.env.Provider.Send(req.From, to, cc, bcc, subject, message)
	if err != nil {
		return nil, err
	}

	if response == "" {
		return nil, fmt.Errorf("failed to send email")
	}

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}

// SendFromTemplate send an email using a predefined template
func (h *mailHandler) SendFromTemplate(ctx context.Context, req *mailpb.SendFromTemplateRequest) (*mailpb.SendResponse, error) {
	log.Printf("Received SendEmailFromTemplate request: %v", req)

	var (
		response string
		err      error
	)

	to, cc, bcc, subject := h.devRedirect(req.To, req.Cc, req.Bcc, req.Subject)

	response, err = h.env.Provider.SendFromTemplate(req.From, to, cc, bcc, subject, req.Template, req.TemplateData)
	if err != nil {
		return nil, err
	}

	if response == "" {
		return nil, fmt.Errorf("failed to send email")
	}

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}
