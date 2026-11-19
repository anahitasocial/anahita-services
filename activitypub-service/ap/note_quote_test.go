package ap

import (
	"encoding/json"
	"testing"
)

// Who may quote a post from elsewhere is read from its canQuote (roadmap
// row 162): the public, approved at once or by hand, is anybody; its
// author's followers are followers; anything else, or nothing said, is
// nobody, as FEP-044f has it.
func TestAPostFromElsewhereSaysWhoMayQuoteIt(t *testing.T) {
	author := "https://mastodon.example/users/alice"
	for policy, want := range map[string]string{
		`{"canQuote":{"automaticApproval":"https://www.w3.org/ns/activitystreams#Public"}}`:    QuoteAnyone,
		`{"canQuote":{"automaticApproval":["` + author + `"],"manualApproval":["as:Public"]}}`: QuoteAnyone,
		`{"canQuote":{"automaticApproval":["` + author + `/followers"]}}`:                      QuoteFollowers,
		`{"canQuote":{"automaticApproval":["` + author + `"]}}`:                                QuoteNobody,
		`{"canReply":{"automaticApproval":["https://www.w3.org/ns/activitystreams#Public"]}}`:  QuoteNobody,
		``: QuoteNobody,
	} {
		n := &ReceivedNote{AttributedTo: json.RawMessage(`"` + author + `"`), InteractionPolicy: json.RawMessage(policy)}
		if got := n.QuotePolicy(); got != want {
			t.Errorf("%s: %s, not %s", policy, got, want)
		}
	}
}
