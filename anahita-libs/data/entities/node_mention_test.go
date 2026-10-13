package entities

import (
	"fmt"
	"testing"
)

func TestParseMentions(t *testing.T) {
	cases := map[string]string{
		"@ana hello":                                "ana",
		"hello @ana and @bob-b":                     "ana bob-b",
		"line one\n@ana":                            "ana",
		"write to ana@example.org":                  "",
		"@alice@mastodon.social hi":                 "alice@mastodon.social",
		"cc @alice@Mastodon.Social.":                "alice@mastodon.social",
		"@alice@mastodon.social, @ana":              "alice@mastodon.social ana",
		"@alice@sub.example.co.uk!":                 "alice@sub.example.co.uk",
		"@alice@localhost":                          "",
		"@alice@ and @ana":                          "ana",
		"@alice@-bad.example":                       "",
		"@alice@mastodon.social@more":               "alice@mastodon.social",
		"no one here":                               "",
		"@ana@example.org is one of ours elsewhere": "ana@example.org",
	}

	for text, want := range cases {
		got := ""
		for _, mention := range ParseMentions(text) {
			if got != "" {
				got += " "
			}
			got += mention.User
			if mention.Host != "" {
				got += "@" + mention.Host
			}
			if text[mention.Start] != '@' {
				t.Errorf("%q: %+v does not begin at its @", text, mention)
			}
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}

	// Where it is in the text is all of it and no more.
	text := "hi @alice@mastodon.social."
	mention := ParseMentions(text)[0]
	if got := text[mention.Start:mention.End]; got != "@alice@mastodon.social" {
		t.Errorf("found at %q", got)
	}
}

// Somebody elsewhere who shares a name with somebody here is not them.
func TestOnlyPeopleHereAreANodesMentions(t *testing.T) {
	t.Setenv("ENV", "prod")
	t.Setenv("DOMAIN_NAME", "example.org")

	node := &AnahitaNode{Body: "@ana and @alice@mastodon.social and @bob@example.org and @ana again, @carol@EXAMPLE.org"}
	if got := fmt.Sprint(node.GetMentions()); got != "[ana bob carol]" {
		t.Errorf("got %s", got)
	}

	if got := (&AnahitaNode{}).GetMentions(); len(got) != 0 {
		t.Errorf("nothing written named %v", got)
	}
}
