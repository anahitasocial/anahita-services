// Package servicecalls is the installation's services calling each other
// over HTTP, where there is no gRPC: for now, import-export-worker asking
// activitypub-service and atproto-service to find a person or a post on
// another server and keep it (roadmap row 124).
//
// On a port of its own, Port, which the gateway never routes, so nothing
// from outside reaches it; and with TLS on, as everywhere but development,
// only a caller holding a certificate from the installation's authority is
// answered (anahita-libs/grpctls). Nobody signed in is involved: whoever
// calls has decided already that the person may.
package servicecalls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"anahita-libs/grpctls"
)

// Port is where the calls are answered.
const Port = "8443"

// The two calls.
const (
	// PeoplePath finds a person by address: alice@mastodon.social, or a
	// Bluesky handle.
	PeoplePath = "/internal/people"
	// PostsPath finds a post by its ActivityPub id or its at:// address.
	PostsPath = "/internal/posts"
)

// Found is the answer: what was found, as kept here.
type Found struct {
	ID int64 `json:"id"`
}

// Refusal is the answer when nothing was found, with why.
type Refusal struct {
	Error string `json:"error"`
}

// What a call can come to, besides an id.
var (
	// ErrNotFound: nobody, or nothing that can be kept, at that address.
	ErrNotFound = errors.New("not found")
	// ErrRefused: on a server, or an account, the installation refuses.
	ErrRefused = errors.New("refused")
	// ErrNotFederating: the installation does not take part in that network.
	ErrNotFederating = errors.New("not federating")
	// ErrFailed: the other server could not be reached or understood.
	ErrFailed = errors.New("failed")
)

// Serve answers handler on Port, and returns only if it cannot.
func Serve(handler http.Handler) error {
	config, err := grpctls.InternalServerConfig()
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              ":" + Port,
		Handler:           handler,
		TLSConfig:         config,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("servicecalls: answering on :%s, TLS %v", Port, config != nil)
	if config != nil {
		return server.ListenAndServeTLS("", "")
	}
	return server.ListenAndServe()
}

// Answer writes a call's answer: the id, or the refusal with its status.
func Answer(w http.ResponseWriter, status int, id int64, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status == http.StatusOK {
		_ = json.NewEncoder(w).Encode(Found{ID: id})
		return
	}
	_ = json.NewEncoder(w).Encode(Refusal{Error: reason})
}

// Client calls the services.
type Client struct {
	http   *http.Client
	scheme string
	// Port, but in this package's tests.
	port string
}

// NewClient is a client presenting this service's certificate, or plain
// HTTP in development.
func NewClient(timeout time.Duration) (*Client, error) {
	config, err := grpctls.InternalClientConfig()
	if err != nil {
		return nil, err
	}
	client := &Client{http: &http.Client{Timeout: timeout}, scheme: "http", port: Port}
	if config != nil {
		client.scheme = "https"
		client.http.Transport = &http.Transport{TLSClientConfig: config.Clone()}
	}
	return client, nil
}

// Person asks the service at host (activitypub-service, atproto-service) to
// find somebody by address and keep them; their node's id here.
func (c *Client) Person(ctx context.Context, host, address string) (int64, error) {
	return c.call(ctx, host, PeoplePath, url.Values{"address": {address}})
}

// Post asks it to find a post by its address and keep it.
func (c *Client) Post(ctx context.Context, host, uri string) (int64, error) {
	return c.call(ctx, host, PostsPath, url.Values{"uri": {uri}})
}

func (c *Client) call(ctx context.Context, host, path string, query url.Values) (int64, error) {
	address := fmt.Sprintf("%s://%s:%s%s?%s", c.scheme, host, c.port, path, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrFailed, err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		var found Found
		if err := json.NewDecoder(res.Body).Decode(&found); err != nil || found.ID == 0 {
			return 0, fmt.Errorf("%w: an answer with no id", ErrFailed)
		}
		return found.ID, nil
	case http.StatusNotFound:
		return 0, ErrNotFound
	case http.StatusForbidden:
		return 0, ErrRefused
	case http.StatusConflict:
		return 0, ErrNotFederating
	default:
		return 0, fmt.Errorf("%w: %s", ErrFailed, res.Status)
	}
}
