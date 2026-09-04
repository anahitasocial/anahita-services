package handlers

import (
	"bytes"
	"html/template"

	"github.com/vanng822/go-premailer/premailer"

	"mail/templates"
)

// Names within the embedded template filesystem — see package mail/templates
// for why these are no longer paths on disk.
const (
	HtmlEmailTemplateName  = "mail.html.tmpl"
	PlainEmailTemplateName = "mail.plain.tmpl"
)

// buildPlainTextMessage creates the plaintext version of the message
func buildPlainTextMessage(data interface{}) (string, error) {
	t, err := template.New("email-plain").ParseFS(templates.FS, PlainEmailTemplateName)
	if err != nil {
		return "", err
	}

	var tplPlain bytes.Buffer
	if err = t.ExecuteTemplate(&tplPlain, "body", data); err != nil {
		return "", err
	}

	plainMessage := tplPlain.String()

	return plainMessage, nil
}

// buildHTMLMessage creates the HTML version of the message
func buildHTMLMessage(data interface{}) (string, error) {
	t, err := template.New("email-html").ParseFS(templates.FS, HtmlEmailTemplateName)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return "", err
	}

	formattedMessage := tpl.String()
	formattedMessage, err = inlineCSS(formattedMessage)
	if err != nil {
		return "", err
	}
	return formattedMessage, nil
}

// inlineCSS takes html input as a string, and inlines css where possible
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
