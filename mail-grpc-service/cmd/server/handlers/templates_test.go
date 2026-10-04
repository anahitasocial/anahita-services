package handlers

import (
	"strings"
	"testing"
)

// The message templates used to live in a Mailgun account, where nothing could
// test them and nothing could tell you what a password reset email looked like.
// Now that they are in the repository, these are the properties worth holding.

func actionData() map[string]any {
	return map[string]any{
		"siteName":    "Banu",
		"logoURL":     "",
		"title":       "Reset your password",
		"body":        "Someone asked to reset the password on your account.",
		"actionLabel": "Choose a new password",
		"actionURL":   "https://auth.example.com/password/reset?token=abc123",
	}
}

func TestRenderTemplateProducesBothParts(t *testing.T) {
	text, html, err := renderTemplate("message_action", actionData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if text == "" {
		t.Error("no text part")
	}
	if html == "" {
		t.Error("no HTML part")
	}

	// All four variables reach both parts. A key silently missing from one of
	// them is the failure this template's whole contract is: every notifier in
	// auth-service passes exactly these four.
	for _, want := range []string{
		"Reset your password",
		"Someone asked to reset the password on your account.",
		"Choose a new password",
		"https://auth.example.com/password/reset?token=abc123",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text part missing %q", want)
		}
		if !strings.Contains(html, want) {
			t.Errorf("HTML part missing %q", want)
		}
	}
}

// The text part is rendered with text/template, not html/template. Both used
// html/template before, which escapes for an HTML context — so an apostrophe
// arrived as &#39; and the ampersands in a link with query parameters were
// mangled. Every one of these messages is prose carrying a tokenised link.
func TestTextPartIsNotHTMLEscaped(t *testing.T) {
	data := actionData()
	data["body"] = "We couldn't verify it & we'd like you to confirm."
	data["actionURL"] = "https://auth.example.com/reset?token=abc&source=email"

	text, _, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(text, "&#39;") || strings.Contains(text, "&amp;") {
		t.Errorf("text part contains HTML entities:\n%s", text)
	}
	if !strings.Contains(text, "couldn't verify it & we'd") {
		t.Error("apostrophes or ampersand did not survive into the text part")
	}
	if !strings.Contains(text, "token=abc&source=email") {
		t.Error("the URL was altered in the text part")
	}
}

// The HTML part, by contrast, MUST escape — a name or a token arriving with a
// stray angle bracket must not become live markup in somebody's inbox.
func TestHTMLPartEscapesInterpolatedContent(t *testing.T) {
	data := actionData()
	data["body"] = `<script>alert("xss")</script>`

	_, html, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(html, "<script>alert") {
		t.Error("script tag reached the HTML part unescaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected the content to be escaped rather than dropped")
	}
}

// premailer moves the <style> rules onto the elements. Gmail and Outlook both
// strip <style> blocks from a delivered message, so a rule that is not inlined
// is simply not applied — the button would arrive as a bare link.
func TestHTMLPartHasInlinedStyles(t *testing.T) {
	_, html, err := renderTemplate("message_action", actionData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(html, "style=") {
		t.Fatal("no inline style attributes; premailer did not run")
	}
	// The action button is the element that most needs it.
	if !strings.Contains(html, "background-color") {
		t.Error("the button's background colour was not inlined")
	}
}

// A template with no action still renders. Not every notification has a button.
func TestRenderTemplateWithoutAnAction(t *testing.T) {
	data := map[string]any{
		"title": "Your password was changed",
		"body":  "This is a confirmation.",
	}

	text, html, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(text, "This is a confirmation.") {
		t.Error("body missing from the text part")
	}
	if strings.Contains(html, "If the button does not work") {
		t.Error("the fallback link block rendered with no action URL")
	}
}

func TestRenderTemplateRejectsUnknownName(t *testing.T) {
	if _, _, err := renderTemplate("does_not_exist", actionData()); err == nil {
		t.Error("expected an error for an unknown template name")
	}
}

// Every template named in the registry must actually exist in the embedded
// filesystem. Without this, a typo in the map is discovered by somebody who
// cannot reset their password.
func TestEveryRegisteredTemplateRenders(t *testing.T) {
	for name := range messageTemplates {
		t.Run(name, func(t *testing.T) {
			if _, _, err := renderTemplate(name, actionData()); err != nil {
				t.Errorf("render: %v", err)
			}
		})
	}
}

// With a logo URL, the header is an image — and the site name survives as its
// alt text, which is what a reader sees while images are still blocked.
func TestHeaderRendersTheLogoWhenConfigured(t *testing.T) {
	data := actionData()
	data["logoURL"] = "https://auth.banu.io/static/images/logo.png"

	text, html, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if !strings.Contains(html, `src="https://auth.banu.io/static/images/logo.png"`) {
		t.Error("the logo URL is not in the HTML part")
	}
	if !strings.Contains(html, `alt="Banu"`) {
		t.Error("the site name is not the logo's alt text; a blocked image would say nothing")
	}

	// The text part has no images, so it always names the site.
	if !strings.Contains(text, "Banu") {
		t.Error("the text part does not name the site")
	}
}

// Without one, the header falls back to the site name as text rather than
// rendering an empty box. This is the default, so it is the common case.
func TestHeaderFallsBackToTheSiteName(t *testing.T) {
	_, html, err := renderTemplate("message_action", actionData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(html, "<img") {
		t.Error("an image was rendered with no logo URL configured")
	}
	if !strings.Contains(html, "Banu") {
		t.Error("the site name does not appear in the header")
	}
}

// The site name is escaped like any other interpolated value — it comes from
// configuration, but configuration is not automatically safe to inject into
// markup, and it lands inside an alt attribute where a stray quote would break
// out of the tag.
func TestSiteNameIsEscapedInTheHeader(t *testing.T) {
	data := actionData()
	data["siteName"] = `Banu" onerror="alert(1)`
	data["logoURL"] = "https://auth.banu.io/logo.png"

	_, html, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if strings.Contains(html, `onerror="alert(1)"`) {
		t.Error("the site name broke out of the alt attribute")
	}
}

// A one-time code gets a block of its own: under the title, above the
// description, boxed and in large type. The styles have to arrive inlined,
// because mail clients drop the <style> block.
func TestCodeIsSetApartBetweenTitleAndBody(t *testing.T) {
	data := map[string]any{
		"siteName": "Anahita",
		"title":    "Your sign-in code",
		"body":     "Enter this code to sign in.",
		"code":     "042195",
	}

	text, html, err := renderTemplate("message_action", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	title := strings.Index(html, "Your sign-in code</h1>")
	code := strings.Index(html, ">042195</td>")
	body := strings.Index(html, "Enter this code to sign in.")
	if title < 0 || code < 0 || body < 0 || !(title < code && code < body) {
		t.Fatalf("order in the HTML part: title %d, code %d, body %d", title, code, body)
	}

	cell := html[strings.LastIndex(html[:code], "<td"):code]
	for _, want := range []string{"border:", "font-size:40px", "text-align:center"} {
		if !strings.Contains(cell, want) {
			t.Errorf("the code's cell is missing %q inline: %s", want, cell)
		}
	}

	// No link and no button in a message that only carries a code.
	if strings.Contains(html, "<a ") {
		t.Error("a code message rendered a link")
	}

	titleText, codeText, bodyText := strings.Index(text, "Your sign-in code"),
		strings.Index(text, "042195"), strings.Index(text, "Enter this code")
	if !(titleText < codeText && codeText < bodyText) || codeText < 0 {
		t.Errorf("order in the text part: %q", text)
	}
}

func TestNoCodeBoxWithoutACode(t *testing.T) {
	_, html, err := renderTemplate("message_action", actionData())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(html, "<td class=\"code\"") || strings.Contains(html, "letter-spacing:10px") {
		t.Error("a message with no code rendered the code box")
	}
}
