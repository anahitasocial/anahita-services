package providers

import (
	"context"
	"encoding/json"
	"mail/config"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

type MailGunMailer struct {
	client   *mailgun.MailgunImpl
	domain   string
	template string
	dataMap  map[string]any
}

// NewMailGunMailer initializes a new MailGunMailer instance
//
// Reads the dedicated MAILGUN_* configuration. It used to take the domain from
// SMTP_HOST and the API key from SMTP_PASSWORD, which meant a Mailgun
// deployment had to populate SMTP settings it never used, and anyone reading
// the config would conclude the service was speaking SMTP.
func NewMailGunMailer(config *config.Config) *MailGunMailer {
	client := mailgun.NewMailgun(config.Mailgun.Domain, config.Mailgun.ApiKey)
	return &MailGunMailer{
		client: client,
		domain: config.Mailgun.Domain,
	}
}

// SendFromTemplate sends an email using a predefined Mailgun template
func (m *MailGunMailer) SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data string) (string, error) {
	m.template = template

	m.dataMap = make(map[string]any)
	err := json.Unmarshal([]byte(data), &m.dataMap)
	if err != nil {
		return "", err
	}

	return m.Send(from, to, cc, bcc, subject, "")
}

// Send sends an email using Mailgun, with or without a template
func (m *MailGunMailer) Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error) {
	// Create a new message using the standalone NewMessage function
	msg := mailgun.NewMessage(from, subject, message)

	// Add recipients
	for _, recipient := range to {
		msg.AddRecipient(recipient)
	}

	for _, recipient := range cc {
		msg.AddCC(recipient)
	}

	for _, recipient := range bcc {
		msg.AddBCC(recipient)
	}

	// If a template is specified, set it and add variables
	if m.template != "" {
		msg.SetTemplate(m.template)

		// Serialize the data to JSON and add it as template variables
		vars, err := json.Marshal(m.dataMap)
		if err != nil {
			return "", err
		}
		msg.AddHeader("X-Mailgun-Variables", string(vars))
	}

	// Set context with timeout for the API request
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Send the email via Mailgun
	_, id, err := m.client.Send(ctx, msg)
	if err != nil {
		return "", err
	}
	return id, nil
}
