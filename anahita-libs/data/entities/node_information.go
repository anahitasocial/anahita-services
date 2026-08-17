package entities

const (
	InformationContactURL   = "contact_url"
	InformationContactEmail = "contact_email"
	InformationPhone        = "phone"
	InformationWebsite      = "website"
)

// GetInformation
func (a *AnahitaNode) GetInformation(fields []string) error {
	var err error
	if len(fields) == 0 {
		fields = []string{
			InformationContactURL,
			InformationContactEmail,
			InformationPhone,
			InformationWebsite,
		}
	}

	// loop through the fields and get the metadata
	a.Information = make(map[string]interface{})
	for _, field := range fields {
		a.Information[field], err = a.GetMetadata(field)
		if err != nil {
			return err
		}
	}

	return nil
}
