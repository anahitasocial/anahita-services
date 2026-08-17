package requests

type MailSendRequest struct {
	From         string   // from email address
	To           []string // to email addresses
	Subject      string   // message subject
	Message      string   // message body
	IsHtml       bool
	Template     string                 // template name or identifier
	TemplateData map[string]interface{} // template data map of key values
}
