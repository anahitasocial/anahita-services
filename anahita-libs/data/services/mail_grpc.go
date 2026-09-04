package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"anahita-libs/proto/mailpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// mailsGRPCMaxConnectAttempts bounds the dial retries for ONE call. It replaces
// a package-level counter that was incremented but never reset — see getClient.
const mailsGRPCMaxConnectAttempts = 10

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

// getClient returns the grpc client
func (srv *MailGRPC) getClient() (mailpb.MailServiceClient, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy": "round_robin"}`))

	// Attempts are counted PER CALL, not for the lifetime of the process.
	//
	// This used to increment a package-level counter that nothing ever reset,
	// so the ten retries were a budget spent once and never refilled: after ten
	// cumulative failures — a mail service that was slow to start, or briefly
	// restarted — every later call in that process returned immediately without
	// dialling, for as long as the service ran. Since every notifier is
	// fire-and-forget, the result was mail silently ceasing to send with
	// nothing to show for it.
	attempts := 0

	for {
		cc, err := grpc.NewClient(srv.config.GetMailServiceAddr(), opts...)

		if err == nil {
			log.Println("Connected to Mail GRPC!")
			return mailpb.NewMailServiceClient(cc), nil
		}

		log.Println("Mail GRPC not ready...")
		attempts++

		if attempts > mailsGRPCMaxConnectAttempts {
			log.Println(err)
			return nil, err
		}

		log.Println("Backing off for two seconds...")
		time.Sleep(2 * time.Second)
	}
}

// Send sends an email
func (srv *MailGRPC) Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error) {
	client, err := srv.getClient()
	if err != nil {
		return "", err
	}

	res, err := client.Send(context.Background(), &mailpb.SendRequest{
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

	_, err = client.SendFromTemplate(context.Background(), &mailpb.SendFromTemplateRequest{
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
