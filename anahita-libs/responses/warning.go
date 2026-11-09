package responses

import "anahita-libs/data/entities"

// Warning is a post's content warning (anahita-services row 122): what it
// holds, so that a reader is shown it behind the warning until they choose
// to see it.
type Warning struct {
	// Its kind: "adult", "suggestive", "graphic", "nudity" or "other".
	Kind string `json:"kind"`
	// An Other warning's words, as its author wrote them. Left out for the
	// other kinds, which are said in the reader's own language.
	Text string `json:"text,omitempty"`
	// That a moderator gave it, which its author may then not take off.
	ByModerator bool `json:"by_moderator,omitempty"`
}

// NewWarning is nil for a post with no warning.
func NewWarning(node *entities.AnahitaNode) *Warning {
	if node == nil || node.Warning == "" {
		return nil
	}
	return &Warning{
		Kind:        node.Warning,
		Text:        node.WarningText,
		ByModerator: node.WarningByModerator,
	}
}
