package providers

import (
	"errors"
	"fmt"
	"mail/config"
	"net/smtp"
	"strings"
)

type SmtpMailer struct {
	host     string
	port     int
	username string
	password string
}

func NewSmtpMailer(cfg *config.Config) *SmtpMailer {
	return &SmtpMailer{
		host:     cfg.Smtp.SmtpHost,
		port:     cfg.Smtp.SmtpPort,
		username: cfg.Smtp.SmtpUser,
		password: cfg.Smtp.SmtpPassword,
	}
}

// SendFromTemplate no implementation for smtp mailer
func (s *SmtpMailer) SendFromTemplate(from string, to []string, subject string, templateStr string, data string) (string, error) {
	return "", errors.New("not implemented for smtp mailer")
}

func (s *SmtpMailer) Send(from string, to []string, subject string, message string) (string, error) {
	// Set up the SMTP server address and authentication
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	// Compose the email headers and body
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=\"utf-8\""

	// Build the email message
	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n" + message)

	// Send the email
	err := smtp.SendMail(addr, auth, from, to, []byte(msgBuilder.String()))
	if err != nil {
		return "", err
	}

	return "Message sent successfully", nil
}
