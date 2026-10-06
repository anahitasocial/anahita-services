package constants

import "strings"

// Who may quote a post: write a note of their own that carries it.
//
//	anyone      anybody who can see it.
//	followers   people who follow its author.
//	nobody      nobody but its author.
//
// A post has its own, when its author says. One that says nothing has its
// author's, kept on the person in the same place. A person who has said
// nothing lets anyone.
const (
	QuotePolicyAnyone    = "anyone"
	QuotePolicyFollowers = "followers"
	QuotePolicyNobody    = "nobody"
)

// NormalizeQuotePolicy checks a value. Empty is valid and means "not said".
func NormalizeQuotePolicy(raw string) (string, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))

	switch raw {
	case "", QuotePolicyAnyone, QuotePolicyFollowers, QuotePolicyNobody:
		return raw, true
	}

	return "", false
}

// EffectiveQuotePolicy is the policy a post is quoted under: its own, or its
// author's, or anyone.
func EffectiveQuotePolicy(post string, author string) string {
	if post != "" {
		return post
	}
	if author != "" {
		return author
	}
	return QuotePolicyAnyone
}

// What a note that quotes a post says about that post when it cannot show
// it.
const (
	// The viewer may not see the quoted post, or it is gone.
	QuoteStateUnavailable = "unavailable"
	// Its author took it out of the quote.
	QuoteStateDetached = "detached"
)
