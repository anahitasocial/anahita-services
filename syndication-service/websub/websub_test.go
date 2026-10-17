package websub

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestACallbackIsSignedForItsWebsite(t *testing.T) {
	sig := Signature(900, "key")
	if len(sig) != 32 || !Signed(900, "key", sig) {
		t.Fatalf("signature %q", sig)
	}
	if Signed(901, "key", sig) || Signed(900, "other", sig) || Signed(900, "", Signature(900, "")) {
		t.Error("a signature was taken for another Website, key, or none")
	}
	api := func(path string) string { return "https://api.example.org" + path }
	if got := Callback(api, 900, "key"); got != "https://api.example.org/websites/_websub/900/"+sig {
		t.Errorf("callback %s", got)
	}
}

type hub struct{ asked url.Values }

func (h *hub) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	h.asked, _ = url.ParseQuery(string(body))
	return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func TestAHubIsAskedToCallBack(t *testing.T) {
	h := &hub{}
	err := Subscribe(context.Background(), &http.Client{Transport: h}, "https://hub.example/", "https://example.com/feed/", "https://api.example.org/websites/_websub/900/x")
	if err != nil {
		t.Fatal(err)
	}
	if h.asked.Get("hub.mode") != "subscribe" || h.asked.Get("hub.topic") != "https://example.com/feed/" ||
		h.asked.Get("hub.callback") != "https://api.example.org/websites/_websub/900/x" || h.asked.Get("hub.lease_seconds") != "864000" {
		t.Errorf("asked %v", h.asked)
	}
}
