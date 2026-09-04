package providers

import (
	"fmt"

	"mail/config"
	"mail/constants"
)

// MailProviderInterface is the interface that all mail providers must implement.
//
// cc and bcc are part of the signature because the proto carries them and the
// Go client sends them. Without them here the handler had nowhere to pass
// req.Cc and req.Bcc, so it dropped both silently — a caller could copy an
// address and watch nothing arrive.
type MailProviderInterface interface {
	SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data string) (string, error)
	Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error)
}

// NewProvider initializes a new mail provider based on the configuration
func NewProvider(config *config.Config) (MailProviderInterface, error) {
	var mailHandler MailProviderInterface
	var err error

	switch config.MailerProvider {
	case constants.MailerProviderAwsSes:
		mailHandler, err = NewAwsSesMailer(config)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS SES mailer: %w", err)
		}
	case constants.MailerProviderMailgun:
		mailHandler = NewMailGunMailer(config)
	case constants.MailerProviderSmtp:
		mailHandler = NewSmtpMailer(config)
	default:
		return nil, fmt.Errorf("unsupported mail provider: %s", config.MailerProvider)
	}

	return mailHandler, nil
}
