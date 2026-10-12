// Package remote is somebody on another server: reading who they are, and
// deciding whether to believe it.
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"activitypub/keys"

	"anahita-libs/federation"
)

// Fetcher is what reads from other servers.
type Fetcher interface {
	Object(ctx context.Context, address string, into any) (string, error)
	JSON(ctx context.Context, address string, into any) (string, error)
}

// Actor is somebody on another server, as far as they are believed.
type Actor struct {
	// ID is their address: what they are called between servers.
	ID string
	// Domain is the server their address is on.
	Domain string
	// Handle is what they are called by people: alice@mastodon.example.
	Handle   string
	Username string
	Name     string
	// Bio is what they say of themselves, as Markdown that is safe to keep.
	Bio string
	// Page is where they are looked at, on their own server.
	Page        string
	Inbox       string
	SharedInbox string
	// Outbox is where what they have posted is listed, or "".
	Outbox string
	// KeyID and PublicKeyPEM are the key they sign with.
	KeyID        string
	PublicKeyPEM string
	// Icon is the address of their avatar there, and Header of the wide
	// picture across the top of their profile. "" for none.
	Icon   string
	Header string
	// Locked says a follow of theirs waits to be approved.
	Locked bool
}

var (
	ErrNotAPerson = errors.New("what is at that address is not somebody who can be followed")
	ErrNotTheirs  = errors.New("the description is not of who it was asked of")
	ErrNoSuchOne  = errors.New("nobody is found by that name")
)

const (
	nameMax = 255
	bioMax  = 5000
)

// The kinds of actor that are kept as a person: somebody, or something that
// posts as somebody would (a bot, an organisation's account).
var personLike = map[string]bool{"Person": true, "Service": true, "Application": true, "Organization": true}

// wire is a description as it arrives. Fields that servers send as one
// thing or as several are kept raw and read leniently.
type wire struct {
	ID                string          `json:"id"`
	Type              json.RawMessage `json:"type"`
	PreferredUsername string          `json:"preferredUsername"`
	Name              string          `json:"name"`
	Summary           string          `json:"summary"`
	URL               json.RawMessage `json:"url"`
	Inbox             string          `json:"inbox"`
	Outbox            string          `json:"outbox"`
	Endpoints         struct {
		SharedInbox string `json:"sharedInbox"`
	} `json:"endpoints"`
	PublicKey struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
	Icon                      json.RawMessage `json:"icon"`
	Image                     json.RawMessage `json:"image"`
	ManuallyApprovesFollowers bool            `json:"manuallyApprovesFollowers"`
}

// Fetch reads who is at an address, and believes only what that server may
// say.
//
// WHAT IS CHECKED, AND WHY. Anybody can run a server and have it say
// anything, so a description is believed only about its own server:
//   - the address it gives for itself is the one it was fetched from, or on
//     the same server: a server cannot describe somebody on another;
//   - its key is its own: named under its own address, on its own server;
//   - its inbox is an https address, so nothing is ever sent in the clear;
//   - it is not one of ours: nothing from outside is believed about this
//     installation.
func Fetch(ctx context.Context, fetcher Fetcher, address string, isOurs func(string) bool) (*Actor, error) {
	if isOurs(address) {
		return nil, ErrNotTheirs
	}

	var w wire
	from, err := fetcher.Object(ctx, address, &w)
	if err != nil {
		return nil, err
	}
	return read(&w, from, isOurs)
}

func read(w *wire, from string, isOurs func(string) bool) (*Actor, error) {
	id, err := httpsURL(w.ID)
	if err != nil || isOurs(w.ID) {
		return nil, ErrNotTheirs
	}
	source, err := httpsURL(from)
	if err != nil || !strings.EqualFold(id.Host, source.Host) {
		return nil, fmt.Errorf("%w: %s says it is %s", ErrNotTheirs, from, w.ID)
	}

	if !personLike[firstString(w.Type)] {
		return nil, ErrNotAPerson
	}

	username := strings.TrimSpace(w.PreferredUsername)
	if username == "" || len(username) > 100 || strings.ContainsAny(username, "@/ \t\n") {
		return nil, ErrNotAPerson
	}

	inbox, err := httpsURL(w.Inbox)
	if err != nil || isOurs(w.Inbox) {
		return nil, fmt.Errorf("%w: no inbox that can be written to", ErrNotAPerson)
	}

	keyID, err := httpsURL(w.PublicKey.ID)
	if err != nil || !strings.EqualFold(keyID.Host, id.Host) || w.PublicKey.Owner != w.ID {
		return nil, fmt.Errorf("%w: the key is not their own", ErrNotTheirs)
	}
	if _, err := keys.ParsePublic(w.PublicKey.PublicKeyPem); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAPerson, err)
	}

	actor := &Actor{
		ID:           w.ID,
		Domain:       strings.ToLower(id.Host),
		Username:     username,
		Handle:       strings.ToLower(username) + "@" + strings.ToLower(id.Host),
		Name:         cut(strings.TrimSpace(w.Name), nameMax),
		Bio:          cut(federation.ToMarkdown(w.Summary), bioMax),
		Inbox:        inbox.String(),
		KeyID:        w.PublicKey.ID,
		PublicKeyPEM: w.PublicKey.PublicKeyPem,
		Locked:       w.ManuallyApprovesFollowers,
	}
	if actor.Name == "" {
		actor.Name = username
	}

	if shared, err := httpsURL(w.Endpoints.SharedInbox); err == nil && !isOurs(w.Endpoints.SharedInbox) {
		actor.SharedInbox = shared.String()
	}
	// Only on their own server: it is fetched, signed, in their name's wake.
	if outbox, err := httpsURL(w.Outbox); err == nil && strings.EqualFold(outbox.Host, id.Host) {
		actor.Outbox = outbox.String()
	}
	if page, err := httpsURL(firstString(w.URL)); err == nil {
		actor.Page = page.String()
	}
	if icon, err := httpsURL(firstURL(w.Icon)); err == nil {
		actor.Icon = icon.String()
	}
	if header, err := httpsURL(firstURL(w.Image)); err == nil {
		actor.Header = header.String()
	}

	return actor, nil
}

// KeyOwner finds whose a key is, from the key's own address.
//
// A request names the key it was signed with, not who signed it. Most
// servers name a key as their owner's address with a fragment
// (…/users/alice#main-key), and fetching that fetches the owner. Some give
// the key an address of its own (…/users/alice/main-key), which answers
// with the key and says whose it is. Either way the owner must be on the
// key's own server: a server cannot lend its users' names to another's key.
func KeyOwner(ctx context.Context, fetcher Fetcher, keyID string) (string, error) {
	key, err := httpsURL(keyID)
	if err != nil {
		return "", ErrNotTheirs
	}

	var found struct {
		ID        string `json:"id"`
		Owner     string `json:"owner"`
		PublicKey struct {
			Owner string `json:"owner"`
		} `json:"publicKey"`
	}
	if _, err := fetcher.Object(ctx, keyID, &found); err != nil {
		return "", err
	}

	owner := found.PublicKey.Owner
	if owner == "" {
		owner = found.Owner
	}
	if owner == "" {
		owner = found.ID
	}

	parsed, err := httpsURL(owner)
	if err != nil || !strings.EqualFold(parsed.Host, key.Host) {
		return "", fmt.Errorf("%w: the key at %s is said to be %s's", ErrNotTheirs, keyID, owner)
	}
	return owner, nil
}

// jrd is a WebFinger answer.
type jrd struct {
	Subject string `json:"subject"`
	Links   []struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"links"`
}

// Resolve finds where a handle points: alice@mastodon.example to the
// address of her description.
func Resolve(ctx context.Context, fetcher Fetcher, username, host string) (string, error) {
	if username == "" || host == "" {
		return "", ErrNoSuchOne
	}

	// Asked in lower case, as every server keeps and compares them.
	query := url.Values{"resource": {"acct:" + strings.ToLower(username) + "@" + strings.ToLower(host)}}
	var found jrd
	if _, err := fetcher.JSON(ctx, "https://"+host+"/.well-known/webfinger?"+query.Encode(), &found); err != nil {
		return "", err
	}

	for _, link := range found.Links {
		if link.Rel != "self" || !strings.Contains(link.Type, "activity+json") && !strings.Contains(link.Type, "ld+json") {
			continue
		}
		if _, err := httpsURL(link.Href); err == nil {
			return link.Href, nil
		}
	}
	return "", ErrNoSuchOne
}

// Lookup finds somebody by their handle, and settles what they are called.
//
// The handle people use and the server a person's address is on need not
// be the same: ana@example.org may live at api.example.org, as people here
// do. A handle is believed when asking its domain who that is leads to this
// very person. Otherwise they are called by the server they are on.
func Lookup(ctx context.Context, fetcher Fetcher, username, host string, isOurs func(string) bool) (*Actor, error) {
	address, err := Resolve(ctx, fetcher, username, host)
	if err != nil {
		return nil, err
	}

	actor, err := Fetch(ctx, fetcher, address, isOurs)
	if err != nil {
		return nil, err
	}

	// The handle that was asked for led here, so it is theirs, as long as
	// the name in it is the one they go by.
	if strings.EqualFold(actor.Username, username) {
		actor.Handle = strings.ToLower(username) + "@" + strings.ToLower(host)
	}
	return actor, nil
}

func httpsURL(address string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("not an https address")
	}
	return parsed, nil
}

// firstString reads a value sent as a string, or as a list whose first
// string is wanted, or as an object with an href.
func firstString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one
	}
	var object struct {
		Href string `json:"href"`
	}
	if json.Unmarshal(raw, &object) == nil && object.Href != "" {
		return object.Href
	}
	var many []json.RawMessage
	if json.Unmarshal(raw, &many) == nil {
		for _, item := range many {
			if found := firstString(item); found != "" {
				return found
			}
		}
	}
	return ""
}

// firstURL reads the address of a picture, sent as an object with a url, a
// list of them, or a bare address.
func firstURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var object struct {
		URL json.RawMessage `json:"url"`
	}
	if json.Unmarshal(raw, &object) == nil && len(object.URL) > 0 {
		return firstString(object.URL)
	}
	var many []json.RawMessage
	if json.Unmarshal(raw, &many) == nil {
		for _, item := range many {
			if found := firstURL(item); found != "" {
				return found
			}
		}
		return ""
	}
	return firstString(raw)
}

// cut is a text no longer than so many characters.
func cut(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
