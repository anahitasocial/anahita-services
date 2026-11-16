package responses

// ContentPreferences is a person's choice for every kind of content
// warning (roadmap row 122, phase 2).
type ContentPreferences struct {
	// By kind (`adult`, `suggestive`, `graphic`, `nudity`, `other`, `ai`):
	// `show`, `warn` or `hide`. Every kind is there, at its default where
	// they have not chosen: `warn`, but `show` for `ai`.
	Choices map[string]string `json:"choices"`
}
