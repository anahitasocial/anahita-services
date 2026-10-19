package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"anahita-libs/proto/mailpb"
)

// mailSendTimeout is how long one email may take to hand to the mail
// service, which has its own timeout for the SMTP server. Without one, a
// stalled mail service held the caller for good.
const mailSendTimeout = 60 * time.Second

type MailGRPCConfigInterface interface {
	GetMailServiceAddr() string
}

// MailGRPCInterface is the interface for mail service
type MailGRPCInterface interface {
	// Send sends an email
	Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error)

	// SendFromTemplate sends an email using a template
	SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data any) error
}

// NewMailGRPC creates a new mail service
func NewMailGRPC(config MailGRPCConfigInterface) MailGRPCInterface {
	return &MailGRPC{
		config: config,
	}
}

// MailGRPC is the mail service
type MailGRPC struct {
	config MailGRPCConfigInterface
}

// getClient returns the client, on the one connection to the mail service
// this process keeps (sharedConnection).
func (srv *MailGRPC) getClient() (mailpb.MailServiceClient, error) {
	conn, err := sharedConnection(srv.config.GetMailServiceAddr())
	if err != nil {
		log.Println("mail: no connection to the mail service:", err)
		return nil, err
	}
	return mailpb.NewMailServiceClient(conn), nil
}

// Send sends an email
func (srv *MailGRPC) Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), mailSendTimeout)
	defer cancel()

	res, err := client.Send(ctx, &mailpb.SendRequest{
		From:    from,
		To:      to,
		Cc:      cc,
		Bcc:     bcc,
		Subject: subject,
		Message: message,
	})
	if err != nil {
		return "", err
	}

	return res.MessageId, nil
}

// SendTemplate sends an email using a template
func (srv *MailGRPC) SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data any) error {
	client, err := srv.getClient()
	if err != nil {
		return err
	}

	templateData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), mailSendTimeout)
	defer cancel()

	_, err = client.SendFromTemplate(ctx, &mailpb.SendFromTemplateRequest{
		From:         from,
		To:           to,
		Cc:           cc,
		Bcc:          bcc,
		Subject:      subject,
		Template:     template,
		TemplateData: string(templateData),
	})
	if err != nil {
		return err
	}

	return nil
}
