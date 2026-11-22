package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// Every API service's server answers /ping, and refuses a body over its
// limit.
func TestTheServerAnswersPingAndKeepsToItsLimit(t *testing.T) {
	e := New(false, "1K")
	e.POST("/echo", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "pong" {
		t.Errorf("ping: %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("x", 2048))))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over the limit: %d", rec.Code)
	}
}

// THE RULE THIS GUARDS (roadmap row 170): told to stop, as Kubernetes
// tells a pod it replaces, a service finishes what it is answering before
// it ends, and takes nothing new.
func TestAServiceFinishesWhatItIsAnsweringBeforeItStops(t *testing.T) {
	e := New(false, "")
	e.HideBanner, e.HidePort = true, true
	started := make(chan struct{})
	e.GET("/slow", func(c echo.Context) error {
		close(started)
		time.Sleep(500 * time.Millisecond)
		return c.String(http.StatusOK, "done")
	})

	stopped := make(chan struct{})
	go func() {
		run(e, "127.0.0.1:0")
		close(stopped)
	}()
	var address string
	for i := 0; i < 100 && address == ""; i++ {
		time.Sleep(10 * time.Millisecond)
		if addr := e.ListenerAddr(); addr != nil {
			address = addr.String()
		}
	}
	if address == "" {
		t.Fatal("the server never started")
	}

	answered := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + address + "/slow")
		if err != nil {
			answered <- err.Error()
			return
		}
		defer resp.Body.Close()
		answered <- resp.Status
	}()
	<-started

	// Told to stop while answering.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-answered:
		if got != "200 OK" {
			t.Errorf("what was being answered: %s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("never answered")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("never stopped")
	}
	if _, err := http.Get("http://" + address + "/ping"); err == nil {
		t.Error("it still takes requests once stopped")
	}
}
