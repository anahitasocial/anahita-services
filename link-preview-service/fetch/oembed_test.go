package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// answering is a transport that answers every request itself, so that
// addresses on YouTube can be tried without going there.
type answering func(w http.ResponseWriter, r *http.Request)

func (a answering) RoundTrip(r *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	a(recorder, r)
	resp := recorder.Result()
	resp.Request = r
	return resp, nil
}

func answeredBy(handler answering) *Fetcher {
	fetcher := New()
	fetcher.client.Transport = handler
	return fetcher
}

// The page a server is given: what is read from it when nothing else
// answers.
const youTubeStub = `<html><head><title> - YouTube</title>
	<meta name="description" content="Enjoy the videos and music that you love">
	</head><body></body></html>`

func TestAYouTubeVideoIsAskedOfYouTube(t *testing.T) {
	for _, address := range []string{
		"https://youtu.be/ncc1701?si=enterprise",
		"https://www.youtube.com/watch?v=ncc1701",
		"https://m.youtube.com/watch?v=ncc1701",
		"https://youtube.com/shorts/ncc1701",
		"https://music.youtube.com/watch?v=ncc1701",
	} {
		fetcher := answeredBy(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.String(), youTubeOEmbed+"?") {
				t.Errorf("%s: went to %s", address, r.URL)
			}
			if r.URL.Query().Get("url") != address || r.URL.Query().Get("format") != "json" {
				t.Errorf("%s: asked %s", address, r.URL.RawQuery)
			}
			if r.Header.Get("User-Agent") != UserAgent {
				t.Errorf("fetched as %q", r.Header.Get("User-Agent"))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"title":"The Inner Light","author_name":"Picard","provider_name":"YouTube",
				"thumbnail_url":"https://i.ytimg.com/vi/ncc1701/hqdefault.jpg"}`)
		})

		page, final, err := fetcher.Page(address)
		if err != nil {
			t.Fatal(err)
		}
		if page.Title != "The Inner Light" || page.Description != "Picard" || page.SiteName != "YouTube" {
			t.Errorf("%s: read %+v", address, page)
		}
		if page.Image != "https://i.ytimg.com/vi/ncc1701/hqdefault.jpg" {
			t.Errorf("%s: picture at %q", address, page.Image)
		}
		if final != address {
			t.Errorf("%s: led to %q", address, final)
		}
	}
}

// A channel's page, a video that has gone: YouTube has no answer, or none
// that can be read, and the page is read as any other is.
func TestThePageIsReadWhenYouTubeHasNoAnswer(t *testing.T) {
	for name, answer := range map[string]func(w http.ResponseWriter){
		"not found": func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) },
		"refused":   func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) },
		"not JSON":  func(w http.ResponseWriter) { fmt.Fprint(w, "<html>") },
		"no title":  func(w http.ResponseWriter) { fmt.Fprint(w, `{"provider_name":"YouTube"}`) },
	} {
		fetcher := answeredBy(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.String(), youTubeOEmbed) {
				answer(w)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, youTubeStub)
		})

		page, _, err := fetcher.Page("https://www.youtube.com/@starfleet")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if page.Title != "- YouTube" {
			t.Errorf("%s: read %+v", name, page)
		}
	}
}

func TestOnlyYouTubeIsAsked(t *testing.T) {
	for _, address := range []string{
		"https://example.org/watch?v=ncc1701",
		"https://youtube.com.example.org/watch?v=ncc1701",
		"https://notyoutube.com/watch?v=ncc1701",
	} {
		fetcher := answeredBy(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.String(), youTubeOEmbed) {
				t.Errorf("%s: YouTube was asked", address)
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><title>Ten Forward</title></head></html>`)
		})

		if _, _, err := fetcher.Page(address); err != nil {
			t.Fatal(err)
		}
	}
}
