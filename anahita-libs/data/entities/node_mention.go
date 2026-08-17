package entities

import "regexp"

const MentionRegexPattern = `(?:^|\s)@([\w-]+)`

type MentionMixinInterface interface {
	GetMentions() []string
}

func (a *AnahitaNode) GetMentions() []string {
	if a.Body == "" {
		return []string{}
	}

	// Regex to find mentions that consist of alphanumeric characters and underscores
	mentionRegex := regexp.MustCompile(MentionRegexPattern)
	matches := mentionRegex.FindAllStringSubmatch(a.Body, -1)

	// Extract mentions from the matches
	a.Mentions = []string{}
	for _, match := range matches {
		if len(match) > 1 { // match[1] contains the mention including the '@'
			a.Mentions = append(a.Mentions, match[1])
		}
	}

	// make sure that m.Mentions has unnique values
	a.Mentions = UniqueString(a.Mentions)

	return a.Mentions
}
