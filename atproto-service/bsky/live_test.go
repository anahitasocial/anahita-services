package bsky

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

// Against Bluesky itself: `ATPROTO_LIVE=1 go test ./bsky/ -run Live -v`.
// Left out of the ordinary run, which asks nothing of the internet.
//
// It reads who a few accounts that post many times a day are, what they
// posted lately, and then listens from six hours back for what they
// posted since, which is what a listener that begins again does.
func TestLiveReadingOfBluesky(t *testing.T) {
	if os.Getenv("ATPROTO_LIVE") == "" {
		t.Skip("ATPROTO_LIVE is not set")
	}

	client := NewClient("https://public.api.bsky.app", "Anahita/1.0 (a test)")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var wanted []string
	for _, handle := range []string{"nytimes.com", "npr.org", "theonion.com"} {
		profile, err := client.Profile(ctx, handle)
		if err != nil {
			t.Fatalf("%s: %v", handle, err)
		}
		t.Logf("%s is %s (%s)", handle, profile.DID, profile.DisplayName)
		wanted = append(wanted, profile.DID)
	}
	if _, err := client.Profile(ctx, "nobody-at-all-0000.bsky.social"); !errors.Is(err, ErrNobody) {
		t.Errorf("somebody who is not there: %v", err)
	}

	feed, err := client.AuthorFeed(ctx, wanted[0], 5)
	if err != nil || len(feed) == 0 {
		t.Fatalf("what %s posted lately: %d, %v", wanted[0], len(feed), err)
	}
	post := &Post{}
	if err := json.Unmarshal(feed[0].Post.Record, post); err != nil || post.HTML() == "" {
		t.Errorf("the first of them: %v, %q", err, post.HTML())
	}
	t.Logf("lately: %s", post.HTML())

	posts := 0
	listening, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	stream := &Stream{
		URLs:      []string{"wss://jetstream1.us-east.bsky.network/subscribe"},
		UserAgent: "Anahita/1.0 (a test)",
		Wanted:    func() ([]string, error) { return wanted, nil },
		Changed:   make(chan struct{}),
		Cursor:    func() int64 { return time.Now().Add(-6 * time.Hour).UnixMicro() },
		Save:      func(at int64) { t.Logf("got to %s", time.UnixMicro(at).UTC().Format(time.RFC3339)) },
		Handle: func(_ context.Context, heard *Event) {
			if heard.Kind != "commit" || heard.Commit.Collection != PostCollection {
				return
			}
			found := false
			for _, did := range wanted {
				found = found || did == heard.DID
			}
			if !found {
				t.Errorf("heard of %s, who was not asked for", heard.DID)
			}
			posts++
			if posts >= 5 {
				stop()
			}
		},
	}
	if err := stream.Run(listening); !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("listening stopped: %v", err)
	}
	if posts == 0 {
		t.Error("nothing was heard in six hours of three newsrooms")
	}
	t.Logf("%d posts heard", posts)
}
