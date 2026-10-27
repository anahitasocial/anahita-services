package providers

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
)

// MIME assembly is hand-rolled, so it is tested by parsing what it produces
// with the standard library rather than by matching strings. A message that
// looks right and does not parse is exactly the failure mode worth catching:
// the recipient sees raw boundary markers, and nothing on our side errors.

func testMessage() Message {
	return Message{
		From:    "system@example.com",
		To:      []string{"alice@example.com"},
		Cc:      []string{"bob@example.com"},
		Bcc:     []string{"carol@example.com"},
		Subject: "Reset your password",
		Text:    "Someone asked to reset your password.\nIf it was not you, ignore this.",
		HTML:    "<html><body><p>Someone asked to reset your password.</p></body></html>",
	}
}

func parseMessage(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("message does not parse: %v", err)
	}
	return msg
}

func TestBuildMIMEParsesAsMultipartAlternative(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	raw, err := s.buildMIME(testMessage())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg := parseMessage(t, raw)

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("content type: %v", err)
	}
	if mediaType != "multipart/alternative" {
		t.Fatalf("media type = %q, want multipart/alternative", mediaType)
	}
	if params["boundary"] == "" {
		t.Fatal("no boundary parameter")
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])

	var types []string
	var bodies []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}

		mt, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("part content type: %v", err)
		}
		types = append(types, mt)

		decoded, err := io.ReadAll(quotedprintable.NewReader(part))
		if err != nil {
			t.Fatalf("decode part: %v", err)
		}
		bodies = append(bodies, string(decoded))
	}

	// Order is required rather than stylistic. RFC 2046 defines the parts of a
	// multipart/alternative as increasing in fidelity and clients render the
	// LAST one they understand — reversed, everybody sees the plain text.
	if len(types) != 2 {
		t.Fatalf("got %d parts, want 2: %v", len(types), types)
	}
	if types[0] != "text/plain" {
		t.Errorf("first part is %q, want text/plain", types[0])
	}
	if types[1] != "text/html" {
		t.Errorf("second part is %q, want text/html", types[1])
	}

	// Quoted-printable must round-trip: this is where a long line gets
	// silently corrupted.
	//
	// Compared with line endings normalised, because the encoder rewrites bare
	// newlines to CRLF and that is correct — RFC 5321 requires CRLF, and a
	// bare LF is what makes some servers reject or truncate a message.
	if normaliseNewlines(bodies[0]) != normaliseNewlines(testMessage().Text) {
		t.Errorf("text part did not survive encoding:\ngot  %q\nwant %q",
			bodies[0], testMessage().Text)
	}
	if normaliseNewlines(bodies[1]) != normaliseNewlines(testMessage().HTML) {
		t.Errorf("HTML part did not survive encoding:\ngot  %q\nwant %q",
			bodies[1], testMessage().HTML)
	}

	// And the encoded form really does use CRLF, which is the property the
	// comparison above deliberately looks past.
	if strings.Contains(strings.ReplaceAll(string(raw), "\r\n", ""), "\n") {
		t.Error("the message contains a bare LF; SMTP requires CRLF")
	}
}

func normaliseNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// Bcc is an envelope recipient and nothing else. A Bcc header is what makes a
// blind copy not blind — every recipient would see the list.
func TestBuildMIMEOmitsBccFromHeaders(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	raw, err := s.buildMIME(testMessage())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg := parseMessage(t, raw)

	if got := msg.Header.Get("Bcc"); got != "" {
		t.Errorf("Bcc header present: %q", got)
	}
	if !strings.Contains(msg.Header.Get("Cc"), "bob@example.com") {
		t.Error("Cc header missing; a visible copy must stay visible")
	}

	// But the envelope still carries them, or the blind copy never arrives.
	recipients := envelopeRecipients(testMessage())
	if len(recipients) != 3 {
		t.Fatalf("envelope has %d recipients, want 3: %v", len(recipients), recipients)
	}
	var found bool
	for _, r := range recipients {
		if r == "carol@example.com" {
			found = true
		}
	}
	if !found {
		t.Error("the bcc recipient is missing from the envelope")
	}
}

// Absent entirely before this rewrite. A missing Message-ID is a spam signal at
// some receivers, and a missing Date is worse — some reject on it outright.
func TestBuildMIMEWritesRequiredHeaders(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	raw, err := s.buildMIME(testMessage())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg := parseMessage(t, raw)

	if _, err := msg.Header.Date(); err != nil {
		t.Errorf("Date header unparseable: %v", err)
	}
	id := msg.Header.Get("Message-ID")
	if !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, ">") {
		t.Errorf("Message-ID = %q, want it wrapped in angle brackets", id)
	}
	if !strings.Contains(id, "@example.com") {
		t.Errorf("Message-ID = %q, want it to carry the sender's domain", id)
	}
	if msg.Header.Get("MIME-Version") != "1.0" {
		t.Error("missing MIME-Version")
	}
}

// Two messages must not share a Message-ID; some receivers treat a repeat as a
// duplicate and drop it silently.
func TestMessageIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := messageID("system@example.com")
		if seen[id] {
			t.Fatalf("duplicate Message-ID: %s", id)
		}
		seen[id] = true
	}
}

// A subject with non-ASCII has to be RFC 2047 encoded or it arrives as mojibake
// — the previous implementation wrote it raw.
func TestBuildMIMEEncodesNonASCIISubject(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	msg := testMessage()
	msg.Subject = "Réinitialisez votre mot de passe"

	raw, err := s.buildMIME(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed := parseMessage(t, raw)
	decoded, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if decoded != msg.Subject {
		t.Errorf("subject = %q, want %q", decoded, msg.Subject)
	}
}

// Text-only is still a valid message — the path taken if a template ever ships
// without an HTML half.
func TestBuildMIMEWithoutHTMLIsPlainText(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	msg := testMessage()
	msg.HTML = ""

	raw, err := s.buildMIME(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed := parseMessage(t, raw)
	mediaType, _, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("content type: %v", err)
	}
	if mediaType != "text/plain" {
		t.Errorf("media type = %q, want text/plain", mediaType)
	}

	body, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if normaliseNewlines(string(body)) != normaliseNewlines(msg.Text) {
		t.Errorf("body = %q, want %q", string(body), msg.Text)
	}
}

// A message with no text part is refused rather than sent: it is a
// deliverability penalty for no benefit, and it means a renderer failed
// silently upstream.
func TestSendRejectsAMessageWithNoTextPart(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	msg := testMessage()
	msg.Text = ""

	if _, err := s.Send(context.Background(), msg); err == nil {
		t.Error("expected an error for a message with no text part")
	}
}

func TestSendRejectsAMessageWithNoRecipients(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	msg := testMessage()
	msg.To, msg.Cc, msg.Bcc = nil, nil, nil

	if _, err := s.Send(context.Background(), msg); err == nil {
		t.Error("expected an error for a message with no recipients")
	}
}

// The From header and the SMTP envelope sender are different strings once a
// display name is involved. `MAIL FROM:<"Example" <system@example.com>>` is a syntax
// error, so the address has to come back apart for the envelope while the
// header keeps the readable form.
func TestEnvelopeSenderStripsTheDisplayName(t *testing.T) {
	cases := []struct {
		from string
		want string
	}{
		{"Example <system@example.com>", "system@example.com"},
		{`"Example" <system@example.com>`, "system@example.com"},
		// A comma in the display name is why the name must be quoted: unquoted,
		// the header parses as two addresses.
		{`"Example, the site" <system@example.com>`, "system@example.com"},
		// RFC 2047-encoded name, as a non-ASCII site produces.
		{"=?utf-8?q?Ex=C3=A4mple?= <system@example.com>", "system@example.com"},
		// The ordinary case: a bare address passes through untouched.
		{"system@example.com", "system@example.com"},
		// Unparseable input is handed to the server as-is, so the rejection
		// comes back with the server's own message rather than a worse one.
		{"not an address", "not an address"},
	}

	for _, tc := range cases {
		t.Run(tc.from, func(t *testing.T) {
			if got := envelopeSender(tc.from); got != tc.want {
				t.Errorf("envelopeSender(%q) = %q, want %q", tc.from, got, tc.want)
			}
		})
	}
}

// The header, meanwhile, must keep the display name — that is the whole point
// of setting one.
func TestFromHeaderKeepsTheDisplayName(t *testing.T) {
	s := &SmtpMailer{host: "smtp.example.com", port: 587}

	msg := testMessage()
	msg.From = "Example <system@example.com>"

	raw, err := s.buildMIME(msg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed := parseMessage(t, raw)
	addr, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil {
		t.Fatalf("From header does not parse: %v", err)
	}
	if addr.Name != "Example" {
		t.Errorf("display name = %q, want Example", addr.Name)
	}
	if addr.Address != "system@example.com" {
		t.Errorf("address = %q, want system@example.com", addr.Address)
	}

	// Message-ID is derived from the sender, and must pick up the domain
	// rather than a fragment of the display name.
	if id := parsed.Header.Get("Message-ID"); !strings.Contains(id, "@example.com") {
		t.Errorf("Message-ID = %q, want the sender's domain", id)
	}
}
