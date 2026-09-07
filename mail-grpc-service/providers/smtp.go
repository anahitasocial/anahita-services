package providers

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"mail/config"
)

// sendTimeout bounds one delivery attempt end to end.
//
// The previous implementation called smtp.SendMail, which takes no context and
// has no deadline of its own, so a server that accepted the connection and then
// stopped responding pinned the RPC goroutine for as long as the process ran.
// Thirty seconds matches what the Mailgun and SES clients used.
const sendTimeout = 30 * time.Second

type SmtpMailer struct {
	host     string
	port     int
	username string
	password string

	// secure selects implicit TLS (the whole session is wrapped, port 465)
	// over STARTTLS (plain connection upgraded in place, port 587).
	//
	// SMTP_SECURE was read into config and exposed through a getter, but
	// NewSmtpMailer never copied it onto the struct and nothing ever consulted
	// it — the setting has been inert since it was added.
	secure bool
}

func NewSmtpMailer(cfg *config.Config) *SmtpMailer {
	return &SmtpMailer{
		host:     cfg.Smtp.SmtpHost,
		port:     cfg.Smtp.SmtpPort,
		username: cfg.Smtp.SmtpUser,
		password: cfg.Smtp.SmtpPassword,
		secure:   cfg.Smtp.SmtpSecure,
	}
}

// Send delivers one message over SMTP.
//
// Works against any SMTP server — a vendor relay, an internal MTA, or a local
// sink like mailpit — which is the whole point of preferring it to a vendor
// API. Authentication is skipped when no username is configured, because an
// internal relay that accepts unauthenticated mail from inside the network is a
// perfectly ordinary arrangement and demanding credentials would exclude it.
func (s *SmtpMailer) Send(ctx context.Context, msg Message) (string, error) {
	if msg.Text == "" {
		return "", errors.New("smtp: message has no text part")
	}

	recipients := envelopeRecipients(msg)
	if len(recipients) == 0 {
		return "", errors.New("smtp: message has no recipients")
	}

	body, err := s.buildMIME(msg)
	if err != nil {
		return "", fmt.Errorf("smtp: build message: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	client, err := s.dial(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return "", fmt.Errorf("smtp: authenticate: %w", err)
		}
	}

	// The ENVELOPE sender, which is not the same string as the From
	// header. A header may carry a display name — `Banu <system@banu.io>`
	// — and MAIL FROM may not: `MAIL FROM:<"Banu" <system@banu.io>>` is a
	// syntax error and the server rejects the message. Callers set a
	// display name to identify the installation, so this has to come
	// apart here rather than being pushed back onto them.
	if err := client.Mail(envelopeSender(msg.From)); err != nil {
		return "", fmt.Errorf("smtp: sender rejected: %w", err)
	}

	// One RCPT TO per envelope recipient. Bcc is in this list and NOT in the
	// headers — that is exactly what makes a blind copy blind.
	for _, addr := range recipients {
		if err := client.Rcpt(addr); err != nil {
			return "", fmt.Errorf("smtp: recipient %q rejected: %w", addr, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return "", fmt.Errorf("smtp: write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp: complete message: %w", err)
	}

	// Quit rather than Close, so the server gets a clean QUIT and a queued
	// message is not left looking abandoned.
	if err := client.Quit(); err != nil {
		return "", fmt.Errorf("smtp: quit: %w", err)
	}

	return "Message sent successfully", nil
}

// dial opens a connection and brings it up to TLS.
//
// Two shapes, and the difference is when the handshake happens: implicit TLS
// negotiates before any SMTP is spoken, STARTTLS opens in the clear and
// upgrades. Both end encrypted; STARTTLS is only offered when the server
// advertises it, and a server that does not is left unencrypted rather than
// failing — that is what allows an in-cluster relay or a local mail sink to
// work at all.
func (s *SmtpMailer) dial(ctx context.Context) (*smtp.Client, error) {
	addr := net.JoinHostPort(s.host, fmt.Sprintf("%d", s.port))
	dialer := &net.Dialer{}

	if s.secure {
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.host})
		if err != nil {
			return nil, fmt.Errorf("smtp: tls dial %s: %w", addr, err)
		}
		client, err := smtp.NewClient(conn, s.host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp: client: %w", err)
		}
		return client, nil
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("smtp: dial %s: %w", addr, err)
	}

	// The deadline covers the whole exchange, not just the dial: without it
	// the context above would stop mattering the moment the connection was
	// established, which is precisely when a server can stall.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smtp: client: %w", err)
	}

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
			client.Close()
			return nil, fmt.Errorf("smtp: starttls: %w", err)
		}
	}

	return client, nil
}

// buildMIME assembles headers plus a multipart/alternative body.
//
// Text part first, HTML second. The order is required rather than stylistic:
// RFC 2046 defines the parts of a multipart/alternative as increasing in
// fidelity, and clients display the LAST one they can render — reversing them
// shows the plain text to everybody.
//
// Both parts are quoted-printable encoded so that non-ASCII content and long
// lines survive servers that enforce the 998-character line limit.
func (s *SmtpMailer) buildMIME(msg Message) ([]byte, error) {
	var buf strings.Builder

	// A fixed order, written directly. The previous implementation built
	// headers in a map and ranged over it, so the order was randomised on
	// every send — some filters read that as a sign of a non-standard client.
	writeHeader(&buf, "From", msg.From)
	writeHeader(&buf, "To", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		writeHeader(&buf, "Cc", strings.Join(msg.Cc, ", "))
	}
	// No Bcc header, ever. See the envelope loop in Send.
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	writeHeader(&buf, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&buf, "Message-ID", messageID(msg.From))
	writeHeader(&buf, "MIME-Version", "1.0")

	if msg.HTML == "" {
		// Text only. Still a valid message, and the path a caller lands on if
		// a template ever ships without an HTML half.
		writeHeader(&buf, "Content-Type", `text/plain; charset="utf-8"`)
		writeHeader(&buf, "Content-Transfer-Encoding", "quoted-printable")
		buf.WriteString("\r\n")
		if err := writeQuotedPrintable(&buf, msg.Text); err != nil {
			return nil, err
		}
		return []byte(buf.String()), nil
	}

	var bodyBuf strings.Builder
	mp := multipart.NewWriter(&bodyBuf)

	writeHeader(&buf, "Content-Type",
		fmt.Sprintf("multipart/alternative; boundary=%q", mp.Boundary()))
	buf.WriteString("\r\n")

	if err := writePart(mp, "text/plain", msg.Text); err != nil {
		return nil, err
	}
	if err := writePart(mp, "text/html", msg.HTML); err != nil {
		return nil, err
	}
	if err := mp.Close(); err != nil {
		return nil, err
	}

	buf.WriteString(bodyBuf.String())

	return []byte(buf.String()), nil
}

func writePart(mp *multipart.Writer, contentType, content string) error {
	headers := textproto.MIMEHeader{}
	headers.Set("Content-Type", contentType+`; charset="utf-8"`)
	headers.Set("Content-Transfer-Encoding", "quoted-printable")

	part, err := mp.CreatePart(headers)
	if err != nil {
		return err
	}

	qp := quotedprintable.NewWriter(part)
	if _, err := qp.Write([]byte(content)); err != nil {
		return err
	}

	return qp.Close()
}

func writeQuotedPrintable(w *strings.Builder, content string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(content)); err != nil {
		return err
	}
	return qp.Close()
}

func writeHeader(w *strings.Builder, name, value string) {
	w.WriteString(name)
	w.WriteString(": ")
	w.WriteString(value)
	w.WriteString("\r\n")
}

// envelopeSender reduces a From header to the bare address SMTP wants.
//
// `Banu <system@banu.io>` becomes `system@banu.io`. Falls back to the input
// unchanged when it does not parse, which covers the ordinary case of a plain
// address and means a malformed one is rejected by the server — with the
// server's own error message — rather than here with a worse one.
func envelopeSender(from string) string {
	if addr, err := mail.ParseAddress(from); err == nil {
		return addr.Address
	}
	return from
}

// envelopeRecipients is everyone who should receive the message, whether or not
// they appear in a header.
func envelopeRecipients(msg Message) []string {
	recipients := make([]string, 0, len(msg.To)+len(msg.Cc)+len(msg.Bcc))
	recipients = append(recipients, msg.To...)
	recipients = append(recipients, msg.Cc...)
	recipients = append(recipients, msg.Bcc...)
	return recipients
}

// messageID builds an RFC 5322 Message-ID.
//
// Absent entirely before this. Some receivers treat a missing Message-ID as a
// spam signal, and without one there is no stable handle for a message in
// anybody's logs — including the recipient's, when they ask why mail did not
// arrive.
func messageID(from string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A predictable id is worth more than no id: the header's job is to be
		// unique enough to trace, and the failure path here is effectively
		// unreachable on a healthy system.
		return fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), hostFromAddress(from))
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b[:]), hostFromAddress(from))
}

func hostFromAddress(address string) string {
	if i := strings.LastIndex(address, "@"); i >= 0 && i < len(address)-1 {
		return strings.Trim(address[i+1:], "<> ")
	}
	return "localhost"
}
