package bsky

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Writing to Bluesky, as an account somebody here connected.
//
// An account is written to at the server that holds it, which is found
// from the account's lasting name. Whoever connects one gives a password
// made for an app, once; it is exchanged there for two tokens, one that
// lasts a couple of hours and is used for each thing written, and one
// that lasts longer and gets the next of the first. The password itself
// is not kept.

var (
	// ErrDenied is the account's server refusing who is asking: a wrong
	// password, or a token it no longer takes.
	ErrDenied = errors.New("the account's server refused")
	// ErrExpired is the short token having run out: the long one gets
	// another.
	ErrExpired = errors.New("the token has expired")
)

// Session is what an account's server gives for a password, or for the
// long token.
type Session struct {
	DID        string `json:"did"`
	Handle     string `json:"handle"`
	AccessJWT  string `json:"accessJwt"`
	RefreshJWT string `json:"refreshJwt"`
}

// Ref names one record and exactly what it said when it was named.
type Ref struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

// PDSOf finds the server that holds an account, from the account's
// lasting name. `directory` is where did:plc names are looked up
// ("https://plc.directory").
func (c *Client) PDSOf(ctx context.Context, directory, did string) (string, error) {
	var address string
	switch {
	case strings.HasPrefix(did, "did:plc:") && ValidDID(did):
		address = directory + "/" + did
	case strings.HasPrefix(did, "did:web:") && ValidDID(did):
		host, err := url.PathUnescape(strings.TrimPrefix(did, "did:web:"))
		if err != nil || strings.ContainsAny(host, "/?#@") {
			return "", fmt.Errorf("%w: %s", ErrUnreadable, did)
		}
		address = "https://" + host + "/.well-known/did.json"
	default:
		return "", fmt.Errorf("%w: %s", ErrUnreadable, did)
	}

	var document struct {
		Service []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Endpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	}
	if err := c.call(ctx, http.MethodGet, address, "", nil, &document); err != nil {
		return "", err
	}
	for _, service := range document.Service {
		if service.Type != "AtprotoPersonalDataServer" || !strings.HasSuffix(service.ID, "#atproto_pds") {
			continue
		}
		parsed, err := url.Parse(service.Endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			break
		}
		return "https://" + parsed.Host, nil
	}
	return "", fmt.Errorf("%w: %s names no server", ErrUnreadable, did)
}

// CreateSession exchanges a password for the two tokens.
func (c *Client) CreateSession(ctx context.Context, pds, identifier, password string) (*Session, error) {
	session := &Session{}
	err := c.call(ctx, http.MethodPost, pds+"/xrpc/com.atproto.server.createSession", "",
		map[string]string{"identifier": identifier, "password": password}, session)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// RefreshSession exchanges the long token for a new pair. The one given
// is spent.
func (c *Client) RefreshSession(ctx context.Context, pds, refreshJWT string) (*Session, error) {
	session := &Session{}
	if err := c.call(ctx, http.MethodPost, pds+"/xrpc/com.atproto.server.refreshSession", refreshJWT, nil, session); err != nil {
		return nil, err
	}
	return session, nil
}

// CreateRecord writes one record among an account's own, by the name
// given. Written twice by one name, it is written once.
func (c *Client) CreateRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string, record any) error {
	err := c.call(ctx, http.MethodPost, pds+"/xrpc/com.atproto.repo.putRecord", accessJWT, map[string]any{
		"repo": did, "collection": collection, "rkey": rkey, "record": record,
	}, nil)
	return err
}

// DeleteRecord removes one of an account's records. One that is not there
// is no error.
func (c *Client) DeleteRecord(ctx context.Context, pds, accessJWT, did, collection, rkey string) error {
	return c.call(ctx, http.MethodPost, pds+"/xrpc/com.atproto.repo.deleteRecord", accessJWT, map[string]any{
		"repo": did, "collection": collection, "rkey": rkey,
	}, nil)
}

// FollowCollection is the kind of record a follow is.
const FollowCollection = "app.bsky.graph.follow"

// OutFollow is a follow as it is written: of which account.
type OutFollow struct {
	Type      string `json:"$type"`
	Subject   string `json:"subject"`
	CreatedAt string `json:"createdAt"`
}

// Follows says whether an account follows another already, asked as the
// account itself, of its own server.
func (c *Client) Follows(ctx context.Context, pds, accessJWT, did string) (bool, error) {
	var profile struct {
		Viewer struct {
			Following string `json:"following"`
		} `json:"viewer"`
	}
	address := pds + "/xrpc/app.bsky.actor.getProfile?" + url.Values{"actor": {did}}.Encode()
	if err := c.call(ctx, http.MethodGet, address, accessJWT, nil, &profile); err != nil {
		return false, err
	}
	return profile.Viewer.Following != "", nil
}

// UploadBlob gives a file to an account's server, to be named in a record
// written afterwards: a picture of a post, or of a link's card.
func (c *Client) UploadBlob(ctx context.Context, pds, accessJWT string, content []byte, mimeType string) (*OutBlob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pds+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Authorization", "Bearer "+accessJWT)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var refused xrpcError
		_ = json.Unmarshal(raw, &refused)
		switch {
		case refused.Error == "ExpiredToken":
			return nil, ErrExpired
		case resp.StatusCode == http.StatusUnauthorized, refused.Error == "InvalidToken":
			return nil, fmt.Errorf("%w: %s", ErrDenied, refused.Error)
		}
		return nil, fmt.Errorf("%w: status %d %s: %s", ErrUnreadable, resp.StatusCode, refused.Error, refused.Message)
	}

	var answer struct {
		Blob *OutBlob `json:"blob"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || answer.Blob == nil || answer.Blob.Ref.Link == "" {
		return nil, fmt.Errorf("%w: no file was named", ErrUnreadable)
	}
	return answer.Blob, nil
}

// RefOf reads what a post is named by now, and the post that began the
// thread it is in (itself, when it began one). ErrNobody when the post is
// not there.
func (c *Client) RefOf(ctx context.Context, uri string) (post, root *Ref, err error) {
	var page struct {
		Posts []struct {
			URI    string `json:"uri"`
			CID    string `json:"cid"`
			Record struct {
				Reply *struct {
					Root Ref `json:"root"`
				} `json:"reply"`
			} `json:"record"`
		} `json:"posts"`
	}
	if err := c.xrpc(ctx, "app.bsky.feed.getPosts", url.Values{"uris": {uri}}, &page); err != nil {
		return nil, nil, err
	}
	for _, each := range page.Posts {
		if each.URI != uri || each.CID == "" {
			continue
		}
		post = &Ref{URI: each.URI, CID: each.CID}
		root = post
		if each.Record.Reply != nil && each.Record.Reply.Root.URI != "" && each.Record.Reply.Root.CID != "" {
			root = &Ref{URI: each.Record.Reply.Root.URI, CID: each.Record.Reply.Root.CID}
		}
		return post, root, nil
	}
	return nil, nil, ErrNobody
}

// call asks a server something as JSON and reads its answer as JSON.
func (c *Client) call(ctx context.Context, method, address, token string, body, into any) error {
	var sent io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		sent = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, address, sent)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var refused xrpcError
		_ = json.Unmarshal(raw, &refused)
		switch {
		case refused.Error == "ExpiredToken":
			return ErrExpired
		case resp.StatusCode == http.StatusUnauthorized,
			refused.Error == "AuthenticationRequired", refused.Error == "InvalidToken", refused.Error == "AccountTakedown":
			return fmt.Errorf("%w: %s", ErrDenied, refused.Error)
		}
		return fmt.Errorf("%w: status %d %s: %s", ErrUnreadable, resp.StatusCode, refused.Error, refused.Message)
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return nil
}

// The letters a record's name is written in, in the order they sort.
const tidLetters = "234567abcdefghijklmnopqrstuvwxyz"

var (
	tidMu   sync.Mutex
	tidLast int64
)

// NewTID is a new name for a record: the time, written so that names sort
// in the order they were made, as Bluesky's own are.
func NewTID() string {
	tidMu.Lock()
	now := time.Now().UnixMicro()
	if now <= tidLast {
		now = tidLast + 1
	}
	tidLast = now
	tidMu.Unlock()

	// 53 bits of time, 10 of which clock made it: thirteen letters of
	// five bits each, the first always small.
	value := uint64(now)<<10 | uint64(now%1024)
	name := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		name[i] = tidLetters[value&31]
		value >>= 5
	}
	return string(name)
}
