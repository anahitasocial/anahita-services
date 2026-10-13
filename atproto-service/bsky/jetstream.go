package bsky

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// What is posted, heard as it happens.
//
// Jetstream is a feed of everything done on the network, which a reader
// narrows to the accounts it wants to hear of. One connection is held
// open, told whose posts are wanted, and told again when that changes.
// Each thing heard has a time; the time of the last one dealt with is
// kept, and a connection that begins again asks for what came after it,
// so that what was posted while nobody listened is still heard.

const (
	// The most accounts one connection may ask to hear of.
	MaxWanted = 10000
	// The most one message may weigh. A post is a few kilobytes.
	maxMessage = 1 << 20
	// How far back a connection that begins again asks from, before the
	// last thing it dealt with: hearing a post twice costs nothing.
	rewind = 5 * time.Second

	pingEvery = 30 * time.Second
	saveEvery = 5 * time.Second
)

// Event is one thing heard.
type Event struct {
	DID string `json:"did"`
	// When it happened, in microseconds: what a connection begins again
	// from.
	TimeUS int64  `json:"time_us"`
	Kind   string `json:"kind"`
	// For kind "commit": a record made, changed or deleted.
	Commit *struct {
		Operation  string          `json:"operation"`
		Collection string          `json:"collection"`
		RKey       string          `json:"rkey"`
		Record     json.RawMessage `json:"record"`
	} `json:"commit"`
	// For kind "account": whether the account is still there.
	Account *struct {
		Active bool   `json:"active"`
		Status string `json:"status"`
	} `json:"account"`
}

// Stream is one listener.
type Stream struct {
	// Where Jetstream is: "wss://jetstream2.us-east.bsky.network/subscribe".
	URL       string
	UserAgent string
	// Wanted is the accounts to hear of, asked at the start and whenever
	// Changed says so.
	Wanted func() ([]string, error)
	// Changed is sent on when who is wanted may have changed.
	Changed <-chan struct{}
	// Cursor is the time of the last thing dealt with, or 0 for none.
	Cursor func() int64
	// Save keeps that time.
	Save func(timeUS int64)
	// Handle deals with one thing heard. One at a time, in the order they
	// came.
	Handle func(ctx context.Context, event *Event)
}

// ErrNobodyWanted is there being nobody to listen for: no connection is
// made, since one that names nobody is sent everybody.
var ErrNobodyWanted = errors.New("nobody on Bluesky is followed from here")

// Run listens until something goes wrong or `ctx` is done, and says what.
// The caller runs it again.
func (s *Stream) Run(ctx context.Context) error {
	wanted, err := s.wanted()
	if err != nil {
		return err
	}
	if len(wanted) == 0 {
		return ErrNobodyWanted
	}

	address := s.URL + "?requireHello=true"
	if cursor := s.Cursor(); cursor > 0 {
		address += "&cursor=" + strconv.FormatInt(cursor-rewind.Microseconds(), 10)
	}
	if _, err := url.Parse(address); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dialCtx, dialed := context.WithTimeout(ctx, Timeout)
	conn, _, err := websocket.Dial(dialCtx, address, &websocket.DialOptions{
		HTTPHeader: http.Header{"User-Agent": {s.UserAgent}},
	})
	dialed()
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(maxMessage)

	if err := s.tell(ctx, conn, wanted); err != nil {
		return err
	}

	// Read beside the rest, so that a connection that has gone quiet is
	// still pinged and still told who is wanted.
	heard := make(chan *Event, 256)
	failed := make(chan error, 1)
	go func() {
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				failed <- err
				return
			}
			event := &Event{}
			if json.Unmarshal(raw, event) != nil {
				continue
			}
			select {
			case heard <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	told := wanted
	var latest, saved int64
	save := func() {
		if latest > saved {
			s.Save(latest)
			saved = latest
		}
	}
	defer save()

	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	keep := time.NewTicker(saveEvery)
	defer keep.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-failed:
			return err

		case event := <-heard:
			s.Handle(ctx, event)
			if event.TimeUS > latest {
				latest = event.TimeUS
			}

		case <-keep.C:
			save()

		case <-ping.C:
			pingCtx, pinged := context.WithTimeout(ctx, Timeout)
			err := conn.Ping(pingCtx)
			pinged()
			if err != nil {
				return fmt.Errorf("the connection went quiet: %w", err)
			}

		case <-s.Changed:
			wanted, err = s.wanted()
			if err != nil {
				return err
			}
			if len(wanted) == 0 {
				_ = conn.Close(websocket.StatusNormalClosure, "")
				return ErrNobodyWanted
			}
			// Said only when it is news: this is asked every minute.
			if slices.Equal(wanted, told) {
				continue
			}
			if err := s.tell(ctx, conn, wanted); err != nil {
				return err
			}
			told = wanted
		}
	}
}

func (s *Stream) wanted() ([]string, error) {
	wanted, err := s.Wanted()
	if err != nil {
		return nil, err
	}
	if len(wanted) > MaxWanted {
		wanted = wanted[:MaxWanted]
	}
	return wanted, nil
}

// tell says whose posts and profiles are wanted.
func (s *Stream) tell(ctx context.Context, conn *websocket.Conn, wanted []string) error {
	message := map[string]any{
		"type": "options_update",
		"payload": map[string]any{
			"wantedCollections":   []string{PostCollection, ProfileCollection},
			"wantedDids":          wanted,
			"maxMessageSizeBytes": maxMessage,
		},
	}
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, raw)
}
