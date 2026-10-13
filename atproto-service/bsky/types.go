package bsky

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Profile is who an account is, as the network's reader tells it.
type Profile struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	// Where its pictures are. Empty for none.
	Avatar string `json:"avatar"`
	Banner string `json:"banner"`
}

// FeedItem is one thing in the list of what an account posted.
type FeedItem struct {
	Post struct {
		URI    string `json:"uri"`
		Author struct {
			DID string `json:"did"`
		} `json:"author"`
		Record json.RawMessage `json:"record"`
	} `json:"post"`
	// Why it is in the list when it is not the account's own post: a
	// repost of somebody else's, or one pinned. Empty for its own.
	Reason json.RawMessage `json:"reason"`
}

// Post is a post as its author wrote it: the record, not what a reader
// makes of it.
type Post struct {
	Text      string   `json:"text"`
	CreatedAt string   `json:"createdAt"`
	Langs     []string `json:"langs"`
	Facets    []Facet  `json:"facets"`
	// What it answers. Nil for a post that answers nothing.
	Reply  json.RawMessage `json:"reply"`
	Embed  *Embed          `json:"embed"`
	Labels *struct {
		Values []struct {
			Val string `json:"val"`
		} `json:"values"`
	} `json:"labels"`
}

// Facet says what a stretch of a post's words is: a link, somebody's
// name, a hashtag. The stretch is counted in bytes of UTF-8.
type Facet struct {
	Index struct {
		ByteStart int `json:"byteStart"`
		ByteEnd   int `json:"byteEnd"`
	} `json:"index"`
	Features []struct {
		Type string `json:"$type"`
		URI  string `json:"uri"`
		DID  string `json:"did"`
		Tag  string `json:"tag"`
	} `json:"features"`
}

// Embed is what a post carries besides its words.
type Embed struct {
	Type string `json:"$type"`
	// app.bsky.embed.images
	Images []struct {
		Alt   string `json:"alt"`
		Image Blob   `json:"image"`
	} `json:"images"`
	// app.bsky.embed.external
	External *struct {
		URI string `json:"uri"`
	} `json:"external"`
	// app.bsky.embed.record: {uri, cid}. app.bsky.embed.recordWithMedia:
	// {record: {uri, cid}}. Read by Quoted.
	Record json.RawMessage `json:"record"`
	// app.bsky.embed.recordWithMedia
	Media *Embed `json:"media"`
}

// Blob is a file an account keeps, named by what it holds.
type Blob struct {
	Ref struct {
		Link string `json:"$link"`
	} `json:"ref"`
	MimeType string `json:"mimeType"`
}

var (
	didPattern = regexp.MustCompile(`^did:(plc:[a-z2-7]{24}|web:[a-zA-Z0-9.%-]{1,200})$`)
	// A handle is a name in the DNS: labels of letters, digits and
	// hyphens, the last not beginning with a digit.
	handlePattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]([a-z0-9-]{0,61}[a-z0-9])?$`)
	rkeyPattern   = regexp.MustCompile(`^[A-Za-z0-9._:~-]{1,512}$`)
	cidPattern    = regexp.MustCompile(`^[a-z0-9]{20,128}$`)
)

// ValidDID says whether something is an account's lasting name.
func ValidDID(did string) bool { return len(did) <= 255 && didPattern.MatchString(did) }

// ValidHandle says whether something could be a handle.
func ValidHandle(handle string) bool { return len(handle) <= 253 && handlePattern.MatchString(handle) }

// PostCollection is the kind of record a post is.
const PostCollection = "app.bsky.feed.post"

// ProfileCollection is the record of what an account says of itself.
const ProfileCollection = "app.bsky.actor.profile"

// RepostCollection is the kind of record a repost is.
const RepostCollection = "app.bsky.feed.repost"

// Repost is a repost as whoever made it wrote it: which post.
type Repost struct {
	Subject struct {
		URI string `json:"uri"`
	} `json:"subject"`
}

// ActorURI is the address an account has between servers.
func ActorURI(did string) string { return "at://" + did }

// PostURI is the address a post has between servers.
func PostURI(did, rkey string) string {
	return "at://" + did + "/" + PostCollection + "/" + rkey
}

// ParsePostURI reads whose a post is and which, from its address. False
// for anything that is not the address of a post.
func ParsePostURI(uri string) (did, rkey string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(uri, "at://"), "/")
	if !strings.HasPrefix(uri, "at://") || len(parts) != 3 || parts[1] != PostCollection {
		return "", "", false
	}
	if !ValidDID(parts[0]) || !rkeyPattern.MatchString(parts[2]) {
		return "", "", false
	}
	return parts[0], parts[2], true
}

// ValidRkey says whether something could be the name of a record.
func ValidRkey(rkey string) bool { return rkeyPattern.MatchString(rkey) }

// The site people read Bluesky on: where "see it there" leads.
const web = "https://bsky.app"

// ProfilePage is an account's page, for a person to look at.
func ProfilePage(did string) string { return web + "/profile/" + did }

// PostPage is a post's page, for a person to look at.
func PostPage(did, rkey string) string { return web + "/profile/" + did + "/post/" + rkey }

// ParseActor reads what somebody typed to find an account: a handle with
// or without its @, a DID, the address of a profile page, or an at://
// address. It answers with the handle or DID to ask for, or "".
func ParseActor(typed string) string {
	typed = strings.TrimSpace(typed)
	for _, prefix := range []string{web + "/profile/", "at://"} {
		if strings.HasPrefix(typed, prefix) {
			typed = strings.TrimPrefix(typed, prefix)
			if slash := strings.IndexAny(typed, "/?#"); slash >= 0 {
				typed = typed[:slash]
			}
		}
	}
	typed = strings.TrimPrefix(typed, "@")

	if ValidDID(typed) {
		return typed
	}
	if handle := strings.ToLower(typed); ValidHandle(handle) {
		return handle
	}
	return ""
}
