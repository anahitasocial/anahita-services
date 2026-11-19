package ap

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
)

// Quotes between servers, as Mastodon makes them (FEP-044f; roadmap row
// 156): a post names what it quotes, and the quoted post's author says, in
// a stamp their server keeps, that they let it. A server asks for the
// stamp with a QuoteRequest and is answered with an Accept that carries it,
// or a Reject. Each post says who may quote it.

// InteractionPolicy says who may do what with a post: here, who may quote
// it, approved without being asked.
type InteractionPolicy struct {
	CanQuote *SubPolicy `json:"canQuote,omitempty"`
}

// SubPolicy is who is approved at once. Anybody not in it is never
// approved: Anahita asks nobody to approve by hand.
type SubPolicy struct {
	AutomaticApproval []string `json:"automaticApproval"`
}

// Who may quote a post, as Anahita's quote policy says it.
const (
	QuoteAnyone    = "anyone"
	QuoteFollowers = "followers"
	QuoteNobody    = "nobody"
)

// WithQuotePolicy says who may quote the post: everybody, its author's
// followers, or nobody but its author, which FEP-044f says by naming the
// author alone. A reply is not quoted here: nobody but its author.
func (n *Note) WithQuotePolicy(policy, followers string, reply bool) *Note {
	approved := []string{n.AttributedTo}
	switch {
	case reply:
	case policy == QuoteAnyone:
		approved = []string{Public}
	case policy == QuoteFollowers && followers != "":
		approved = []string{followers}
	}
	n.InteractionPolicy = &InteractionPolicy{CanQuote: &SubPolicy{AutomaticApproval: approved}}
	return n
}

// QuotesPost makes the post a quote of another between servers: `quoted`,
// the quoted post's address between servers, under each name servers read
// it by; `stamp`, the address of its author's approval, when there is one;
// and, for a server that shows no quotes, a line that leads to it, `page`,
// which Mastodon hides when it shows the quote.
func (n *Note) QuotesPost(quoted, stamp, page string) *Note {
	n.Quote, n.QuoteURI, n.MisskeyQuote = quoted, quoted, quoted
	n.QuoteAuthorization = stamp
	if page == "" {
		page = quoted
	}
	line := fmt.Sprintf(`<p class="quote-inline">RE: <a href="%s">%s</a></p>`,
		html.EscapeString(page), html.EscapeString(page))
	n.Content += line
	for language, words := range n.ContentMap {
		n.ContentMap[language] = words + line
	}
	return n
}

// QuoteAuthorization is a stamp: `attributedTo`, the quoted post's author,
// let `interactingObject` quote `interactionTarget`.
type QuoteAuthorization struct {
	Context           any    `json:"@context,omitempty"`
	ID                string `json:"id"`
	Type              string `json:"type"`
	AttributedTo      string `json:"attributedTo"`
	InteractingObject string `json:"interactingObject"`
	InteractionTarget string `json:"interactionTarget"`
}

// NewQuoteAuthorization is a stamp at `id`.
func NewQuoteAuthorization(id, author, quote, quoted string) *QuoteAuthorization {
	return &QuoteAuthorization{
		Context: quoteContext, ID: id, Type: "QuoteAuthorization",
		AttributedTo: author, InteractingObject: quote, InteractionTarget: quoted,
	}
}

// ValidFor says whether a stamp approves `quote` quoting `quoted`, written
// by `author`: what FEP-044f asks a server to check of one.
func (q *QuoteAuthorization) ValidFor(quote, quoted, author string) bool {
	return q != nil && q.Type == "QuoteAuthorization" &&
		q.InteractingObject == quote && q.InteractionTarget == quoted && q.AttributedTo == author
}

// QuoteRequest asks `object`'s author to let `instrument` quote it.
type QuoteRequest struct {
	Context    any    `json:"@context,omitempty"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	Actor      string `json:"actor"`
	Object     string `json:"object"`
	Instrument any    `json:"instrument"`
}

// NewQuoteRequest asks for `note`, a quote, to be let quote `quoted`.
func NewQuoteRequest(id string, note *Note, quoted string) *QuoteRequest {
	return &QuoteRequest{Context: quoteContext, ID: id, Type: "QuoteRequest", Actor: note.AttributedTo, Object: quoted, Instrument: note}
}

// QuoteAnswer is the Accept or the Reject of a QuoteRequest; an Accept
// carries the stamp as its result.
type QuoteAnswer struct {
	Context any               `json:"@context"`
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Actor   string            `json:"actor"`
	To      string            `json:"to"`
	Object  *QuoteRequestSeen `json:"object"`
	Result  string            `json:"result,omitempty"`
}

// QuoteRequestSeen is a QuoteRequest as it is answered: quoted back, with
// its quote by its address alone.
type QuoteRequestSeen struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Actor      string `json:"actor"`
	Object     string `json:"object"`
	Instrument string `json:"instrument"`
}

// NewQuoteAnswer answers a QuoteRequest: an Accept with `stamp`, or a
// Reject without one.
func NewQuoteAnswer(id, actor string, request *QuoteRequestSeen, accepted bool, stamp string) *QuoteAnswer {
	answer := &QuoteAnswer{Context: quoteContext, ID: id, Type: "Reject", Actor: actor, To: request.Actor, Object: request}
	if accepted {
		answer.Type, answer.Result = "Accept", stamp
	}
	return answer
}

// ReadQuoteRequest reads a QuoteRequest as it arrives: its quote inlined or
// by its address.
func ReadQuoteRequest(body []byte) (*QuoteRequestSeen, error) {
	var raw struct {
		ID         string          `json:"id"`
		Type       string          `json:"type"`
		Actor      json.RawMessage `json:"actor"`
		Object     json.RawMessage `json:"object"`
		Instrument json.RawMessage `json:"instrument"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	seen := &QuoteRequestSeen{
		ID: raw.ID, Type: raw.Type, Actor: idOf(raw.Actor), Object: idOf(raw.Object), Instrument: idOf(raw.Instrument),
	}
	if seen.Type != "QuoteRequest" || seen.ID == "" || seen.Actor == "" ||
		!strings.HasPrefix(seen.Object, "https://") || !strings.HasPrefix(seen.Instrument, "https://") {
		return nil, fmt.Errorf("not a QuoteRequest that can be answered")
	}
	return seen, nil
}

// QuotePolicy is who may quote a post from elsewhere, as Anahita's quote
// policy says it (roadmap row 162): everybody, when its `canQuote` names
// the public, approved at once or by hand; its author's followers, when it
// names their collection; and otherwise nobody, as FEP-044f has it: an
// actor named nowhere is expected never to be approved, and a post that
// says nothing is taken to let nobody, as Mastodon takes it. The policy
// says only whether Quote is offered; what lets a quote go out is the
// stamp its author's server gives.
func (n *ReceivedNote) QuotePolicy() string {
	var policy struct {
		CanQuote *struct {
			AutomaticApproval json.RawMessage `json:"automaticApproval"`
			ManualApproval    json.RawMessage `json:"manualApproval"`
		} `json:"canQuote"`
	}
	if len(n.InteractionPolicy) == 0 || json.Unmarshal(n.InteractionPolicy, &policy) != nil || policy.CanQuote == nil {
		return QuoteNobody
	}
	author := n.Author()
	followers := false
	for _, raw := range []json.RawMessage{policy.CanQuote.AutomaticApproval, policy.CanQuote.ManualApproval} {
		for _, who := range idsOf(raw) {
			switch {
			case who == Public || who == "as:Public" || who == "Public":
				return QuoteAnyone
			case author != "" && (who == author+"/followers" || (sameHostAs(who, author) && strings.HasSuffix(who, "/followers"))):
				followers = true
			}
		}
	}
	if followers {
		return QuoteFollowers
	}
	return QuoteNobody
}

// idsOf reads one address or a list of them, each as itself or as the id
// of an object.
func idsOf(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		list = []json.RawMessage{raw}
	}
	ids := make([]string, 0, len(list))
	for _, each := range list {
		if id := idOf(each); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// sameHostAs says whether two addresses are on one server.
func sameHostAs(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	return errA == nil && errB == nil && ua.Host != "" && strings.EqualFold(ua.Host, ub.Host)
}
