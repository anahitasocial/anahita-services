package servicecalls

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// THE RULE THIS GUARDS: what a service answers comes back to the caller as
// an id or as the reason there is none, whatever the reason is.
func TestTheAnswersComeBack(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+PeoplePath, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("address") {
		case "naomi@rocinante.example":
			Answer(w, http.StatusOK, 42, "")
		case "refused@spam.example":
			Answer(w, http.StatusForbidden, 0, "lookup_refused")
		case "away@quiet.example":
			Answer(w, http.StatusConflict, 0, "site_private")
		case "broken@down.example":
			Answer(w, http.StatusBadGateway, 0, "lookup_failed")
		default:
			Answer(w, http.StatusNotFound, 0, "lookup_not_found")
		}
	})
	mux.HandleFunc("GET "+PostsPath, func(w http.ResponseWriter, r *http.Request) {
		Answer(w, http.StatusOK, 7, "")
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	host, port, _ := net.SplitHostPort(server.Listener.Addr().String())

	client := &Client{http: &http.Client{Timeout: time.Second}, scheme: "http", port: port}
	ctx := context.Background()

	if id, err := client.Person(ctx, host, "naomi@rocinante.example"); err != nil || id != 42 {
		t.Errorf("found: %d %v", id, err)
	}
	for address, want := range map[string]error{
		"nobody@rocinante.example": ErrNotFound,
		"refused@spam.example":     ErrRefused,
		"away@quiet.example":       ErrNotFederating,
		"broken@down.example":      ErrFailed,
	} {
		if _, err := client.Person(ctx, host, address); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", address, err, want)
		}
	}
	if id, err := client.Post(ctx, host, "https://rocinante.example/notes/1"); err != nil || id != 7 {
		t.Errorf("post: %d %v", id, err)
	}
}
