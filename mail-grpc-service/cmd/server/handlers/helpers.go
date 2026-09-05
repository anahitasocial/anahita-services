package handlers

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"

	"github.com/vanng822/go-premailer/premailer"

	"mail/providers"
	"mail/templates"
)

// Message templates, by the name callers pass over the wire.
//
// This map is what replaced Mailgun's hosted templates. The name in a
// SendFromTemplate request used to be forwarded verbatim to the vendor and
// resolved in their dashboard, which meant the rendering of every email this
// system sends lived outside version control — nothing here could tell you what
// a message looked like, or whether the template still existed.
//
// Every notifier in auth-service uses "message_action". It is one template
// because all sixteen call sites pass the same four variables: title, body,
// actionLabel and actionURL.
var messageTemplates = map[string]templatePair{
	"message_action": {
		html: "message_action.html.tmpl",
		text: "message_action.txt.tmpl",
	},
}

type templatePair struct {
	html string
	text string
}

// renderTemplate produces both bodies for a named template.
//
// Both, always. A multipart/alternative message needs the pair, and building
// only the one the caller asked for is what the old code did — it had an
// is_html flag and chose. There is nothing to choose: the client decides which
// part to display, which is the point of sending both.
func renderTemplate(name string, data map[string]any) (text string, html string, err error) {
	pair, ok := messageTemplates[name]
	if !ok {
		// Loudly, rather than forwarding an unknown name to a provider and
		// finding out from a delivery failure — or worse, not finding out,
		// since every notifier is fire-and-forget.
		return "", "", fmt.Errorf("unknown mail template %q", name)
	}

	text, err = buildPlainTextMessage(pair.text, data)
	if err != nil {
		return "", "", fmt.Errorf("render %s: %w", pair.text, err)
	}

	html, err = buildHTMLMessage(pair.html, data)
	if err != nil {
		return "", "", fmt.Errorf("render %s: %w", pair.html, err)
	}

	return text, html, nil
}

// buildPlainTextMessage creates the plaintext version of the message.
//
// text/template, NOT html/template. Both templates were previously parsed with
// html/template, which escapes its output for an HTML context — so an
// apostrophe in a message body arrived in the plain-text part as &#39; and a
// URL with query parameters had its ampersands mangled. Every one of these
// messages is written in prose and carries a link with a token in it.
func buildPlainTextMessage(name string, data map[string]any) (string, error) {
	t, err := texttemplate.New("email-plain").ParseFS(templates.FS, name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "body", data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// buildHTMLMessage creates the HTML version of the message.
//
// html/template here is correct and load-bearing: the body and the action URL
// are interpolated into markup, and a person's name or a token arriving with a
// stray angle bracket must not become live HTML in somebody's inbox.
func buildHTMLMessage(name string, data map[string]any) (string, error) {
	t, err := htmltemplate.New("email-html").ParseFS(templates.FS, name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "body", data); err != nil {
		return "", err
	}

	return inlineCSS(buf.String())
}

// inlineCSS takes html input as a string, and inlines css where possible.
//
// Gmail and Outlook both strip <style> blocks from a delivered message, so a
// stylesheet that is not inlined is not applied. This has been wired up since
// before there was a template with any CSS in it to inline; it starts doing
// something now.
func inlineCSS(s string) (string, error) {
	options := premailer.Options{
		RemoveClasses:     false,
		CssToAttributes:   false,
		KeepBangImportant: true,
	}
	prem, err := premailer.NewPremailerFromString(s, &options)
	if err != nil {
		return "", err
	}

	html, err := prem.Transform()
	if err != nil {
		return "", err
	}

	return html, nil
}

// templateData converts the JSON payload a caller sent into template data and
// adds the installation's own branding.
//
// The wire format is map[string]string because that is what every notifier
// builds; widening it to any here keeps the templates able to take richer data
// later without a proto change.
//
// siteName and logoURL come from THIS service's config rather than from the
// caller, and are written last so a caller cannot override them. Branding is a
// property of the installation, not of whichever service sent the message: a
// second sender should not have to remember to brand its mail, and should not
// be able to brand it differently from the first.
func (h *mailHandler) templateData(raw map[string]string) map[string]any {
	data := make(map[string]any, len(raw)+2)
	for k, v := range raw {
		data[k] = v
	}

	data["siteName"] = h.env.Config.GetSiteName()
	data["logoURL"] = h.env.Config.GetSiteLogoURL()

	return data
}

// toMessage assembles a provider Message from the parts the handler has.
func toMessage(from string, to, cc, bcc []string, subject, text, html string) providers.Message {
	return providers.Message{
		From:    from,
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}
