package providers

import (
	"context"
	"encoding/json"
	"mail/config"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

type AwsSesMailer struct {
	client   *ses.Client
	template string
	dataMap  map[string]any
}

func NewAwsSesMailer(config *config.Config) (*AwsSesMailer, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(config.Aws.Region))
	if err != nil {
		return nil, err
	}
	return &AwsSesMailer{
		client: ses.NewFromConfig(awsCfg),
	}, nil
}

func (s *AwsSesMailer) SendFromTemplate(from string, to []string, cc []string, bcc []string, subject string, template string, data string) (string, error) {
	s.template = template
	s.dataMap = make(map[string]any)
	err := json.Unmarshal([]byte(data), &s.dataMap)
	if err != nil {
		return "", err
	}

	return s.Send(from, to, cc, bcc, subject, "")
}

func (s *AwsSesMailer) Send(from string, to []string, cc []string, bcc []string, subject string, message string) (string, error) {
	// SES takes the three lists separately and handles the blind part itself —
	// BccAddresses never reaches the other recipients' headers.
	destination := &types.Destination{
		ToAddresses:  to,
		CcAddresses:  cc,
		BccAddresses: bcc,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.template != "" {
		// Handle templated email
		templateData, err := json.Marshal(s.dataMap)
		if err != nil {
			return "", err
		}

		input := &ses.SendTemplatedEmailInput{
			Source:       aws.String(from),
			Destination:  destination,
			Template:     aws.String(s.template),
			TemplateData: aws.String(string(templateData)),
		}

		// Send templated email
		resp, err := s.client.SendTemplatedEmail(ctx, input)
		if err != nil {
			return "", err
		}
		return *resp.MessageId, nil

	} else {
		// Handle non-templated email
		input := &ses.SendEmailInput{
			Source:      aws.String(from),
			Destination: destination,
			Message: &types.Message{
				Subject: &types.Content{Data: aws.String(subject)},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(message)},
				},
			},
		}

		// Send non-templated email
		resp, err := s.client.SendEmail(ctx, input)
		if err != nil {
			return "", err
		}
		return *resp.MessageId, nil
	}
}
