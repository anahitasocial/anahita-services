// Package bsky reads Bluesky: who somebody is, what they posted, and what
// they post as it happens. Nothing here writes to it.
package bsky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"anahita-libs/safehttp"
)

// Timeout is how long one request to Bluesky is waited for.
const Timeout = 10 * time.Second

const (
	// The most an answer may weigh. A page of twenty posts is well under.
	maxAnswer = 2 << 20
	// How often a picture's address is followed on to another.
	maxRedirects = 3
)

var (
	// ErrNobody is Bluesky saying no such account is there to be read:
	// never made, closed, taken down, or put away by its owner.
	ErrNobody = errors.New("nobody is there")
	// ErrUnreadable is an answer that could not be made sense of.
	ErrUnreadable = errors.New("the answer could not be read")
	// ErrNotAllowed is an address that is not gone to.
	ErrNotAllowed = safehttp.ErrNotAllowed
)

// Client asks the public reader of the network. It needs no account, and
// goes only to public addresses.
type Client struct {
	appView   string
	userAgent string
	http      *http.Client
}

// NewClient asks the reader at `appView` ("https://public.api.bsky.app").
func NewClient(appView, userAgent string) *Client {
	return &Client{
		appView:   appView,
		userAgent: userAgent,
		http:      safehttp.NewClient(safehttp.Options{Timeout: Timeout}),
	}
}

// Profile reads who an account is, by its handle or its DID.
func (c *Client) Profile(ctx context.Context, actor string) (*Profile, error) {
	profile := &Profile{}
	err := c.xrpc(ctx, "app.bsky.actor.getProfile", url.Values{"actor": {actor}}, profile)
	if err != nil {
		return nil, err
	}
	if !ValidDID(profile.DID) {
		return nil, fmt.Errorf("%w: no DID", ErrUnreadable)
	}
	return profile, nil
}

// AuthorFeed reads what an account posted lately, newest first: its own
// posts, without its replies.
func (c *Client) AuthorFeed(ctx context.Context, did string, limit int) ([]FeedItem, error) {
	var page struct {
		Feed []FeedItem `json:"feed"`
	}
	err := c.xrpc(ctx, "app.bsky.feed.getAuthorFeed", url.Values{
		"actor":  {did},
		"limit":  {strconv.Itoa(limit)},
		"filter": {"posts_no_replies"},
	}, &page)
	return page.Feed, err
}

// What Bluesky says when it will not answer.
type xrpcError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (c *Client) xrpc(ctx context.Context, method string, query url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.appView+"/xrpc/"+method+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer+1))
	if err != nil {
		return err
	}
	if len(raw) > maxAnswer {
		return fmt.Errorf("%w: more than %d bytes", ErrUnreadable, maxAnswer)
	}

	if resp.StatusCode == http.StatusBadRequest {
		// An account that is not there is refused as a bad request, with
		// why. Anything else refused that way is ours to have asked better.
		var refused xrpcError
		_ = json.Unmarshal(raw, &refused)
		switch refused.Error {
		case "AccountTakedown", "AccountDeactivated", "AccountSuspended", "NotFound":
			return ErrNobody
		case "InvalidRequest":
			return fmt.Errorf("%w: %s", ErrNobody, refused.Message)
		}
		return fmt.Errorf("%w: %s: %s", ErrUnreadable, refused.Error, refused.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ErrUnreadable, resp.StatusCode)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	return nil
}

// Bytes fetches a picture: https only, to public addresses only, following
// where it is sent a few times, and never more than `max` bytes. It answers
// with what kind of file the server said it is, which is not to be believed
// without looking.
func (c *Client) Bytes(ctx context.Context, address string, max int64) ([]byte, string, error) {
	for hop := 0; hop <= maxRedirects; hop++ {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return nil, "", fmt.Errorf("%w: %s", ErrNotAllowed, address)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrNotAllowed, err)
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, "", err
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			next, err := resp.Location()
			resp.Body.Close()
			if err != nil {
				return nil, "", fmt.Errorf("%w: sent on to nowhere", ErrUnreadable)
			}
			address = next.String()
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("%w: status %d from %s", ErrUnreadable, resp.StatusCode, parsed.Host)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
		if err != nil {
			return nil, "", err
		}
		if int64(len(body)) > max {
			return nil, "", fmt.Errorf("%w: more than %d bytes", ErrUnreadable, max)
		}
		return body, resp.Header.Get("Content-Type"), nil
	}
	return nil, "", fmt.Errorf("%w: sent on too many times", ErrUnreadable)
}
