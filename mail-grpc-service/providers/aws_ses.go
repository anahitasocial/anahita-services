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

func (s *AwsSesMailer) SendFromTemplate(from string, to []string, subject string, template string, data string) (string, error) {
	s.template = template
	s.dataMap = make(map[string]any)
	err := json.Unmarshal([]byte(data), &s.dataMap)
	if err != nil {
		return "", err
	}

	return s.Send(from, to, subject, "")
}

func (s *AwsSesMailer) Send(from string, to []string, subject string, message string) (string, error) {
	// Define `destinations` explicitly as []*string
	destinations := make([]string, len(to))
	for i, recipient := range to {
		destinations[i] = recipient
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.template != "" {
		// Handle templated email
		templateData, err := json.Marshal(s.dataMap)
		if err != nil {
			return "", err
		}

		// Use destinations directly in the struct for ToAddresses
		input := &ses.SendTemplatedEmailInput{
			Source:       aws.String(from),
			Destination:  &types.Destination{ToAddresses: destinations},
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
			Source: aws.String(from),
			Destination: &types.Destination{
				ToAddresses: destinations,
			},
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
