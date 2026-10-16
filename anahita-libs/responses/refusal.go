package responses

// Refusal is why a request was refused, where the answer says: most
// refusals have no body, and some send this.
type Refusal struct {
	// Why, as a code a client can tell apart: `language_invalid`,
	// `quote_not_allowed` and so on. Each route lists its own.
	Error string `json:"error"`
	// The same in words, for whoever is building a client.
	Description string `json:"description,omitempty"`
	// What lies behind it, where one code has several causes: why a post
	// may not be quoted, for one.
	Reason string `json:"reason,omitempty"`
}

// AudienceRefusal refuses an audience a post may not be given, and says
// which it may.
type AudienceRefusal struct {
	// Always `access_not_allowed`.
	Error string `json:"error"`
	// The audiences it may be given, widest first.
	Allowed []string `json:"allowed"`
}
