package entities

import "regexp"

// This regex also supports emojis
const HashtagRegexPattern = `(?:^|\s)#([\w\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}]+)`

// Compiled once: every node read from the graph is looked through for its
// hashtags, its owner, parent and author too (roadmap row 170).
var hashtagRegex = regexp.MustCompile(HashtagRegexPattern)

type HashtagMixinInterface interface {
	GetHashtags() []string
}

func (a *AnahitaNode) GetHashtags() []string {
	if a.Body == "" {
		return []string{}
	}

	matches := hashtagRegex.FindAllStringSubmatch(a.Body, -1)

	// Extract hashtags from the matches
	a.Hashtags = []string{}
	for _, match := range matches {
		if len(match) > 1 { // match[1] contains the hashtag including the '#'
			a.Hashtags = append(a.Hashtags, match[1])
		}
	}

	// make sure that a.Hashtags has unnique values
	a.Hashtags = UniqueString(a.Hashtags)

	return a.Hashtags
}
