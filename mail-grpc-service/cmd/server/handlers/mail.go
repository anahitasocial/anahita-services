package handlers

import (
	"anahita-libs/proto/mailpb"
	"context"
	"fmt"
	"log"
)

type mailHandler struct {
	mailpb.MailServiceServer
	env *Env
}

func NewMailHandler(env *Env) *mailHandler {
	return &mailHandler{env: env}
}

// Send send an email
func (h *mailHandler) Send(ctx context.Context, req *mailpb.SendRequest) (*mailpb.SendResponse, error) {
	log.Printf("Received SendEmail request: %v", req)

	var (
		response string
		err      error
		message  string
	)

	if req.IsHtml {
		message, err = buildHTMLMessage(req.Message)
		if err != nil {
			return nil, err
		}
	} else {
		message, err = buildPlainTextMessage(req.Message)
		if err != nil {
			return nil, err
		}
	}

	response, err = h.env.Provider.Send(req.From, req.To, req.Subject, message)
	if err != nil {
		return nil, err
	}

	if response == "" {
		return nil, fmt.Errorf("failed to send email")
	}

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}

// SendFromTemplate send an email using a predefined template
func (h *mailHandler) SendFromTemplate(ctx context.Context, req *mailpb.SendFromTemplateRequest) (*mailpb.SendResponse, error) {
	log.Printf("Received SendEmailFromTemplate request: %v", req)

	var (
		response string
		err      error
	)

	response, err = h.env.Provider.SendFromTemplate(req.From, req.To, req.Subject, req.Template, req.TemplateData)
	if err != nil {
		return nil, err
	}

	if response == "" {
		return nil, fmt.Errorf("failed to send email")
	}

	return &mailpb.SendResponse{
		Status: response,
	}, nil
}
