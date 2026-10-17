package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pulse/config"
	"pulse/data/repositories"
	"pulse/data/services"
	"pulse/websub"

	"github.com/labstack/echo/v4"
)

type hubPulses struct {
	repositories.PulsesInterface
	hub      string
	verified *time.Time
	soon     int
}

func (h *hubPulses) Feed(context.Context, int64) (string, string, error) {
	return "https://example.com/feed/", h.hub, nil
}
func (h *hubPulses) HubVerified(_ context.Context, _ int64, until time.Time) error {
	h.verified = &until
	return nil
}
func (h *hubPulses) ReadSoon(context.Context, int64) error {
	h.soon++
	return nil
}

func hubCall(h *websubHandler, handler echo.HandlerFunc, method, signature, query string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(method, "/?"+query, strings.NewReader("<feed>anything</feed>"))
	req.Header.Set(echo.HeaderContentType, "application/atom+xml")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id", "signature")
	c.SetParamValues("900", signature)
	_ = handler(c)
	return rec
}

func TestAHubChecksAndCallsBack(t *testing.T) {
	pulses := &hubPulses{hub: "https://hub.example/"}
	h := &websubHandler{&Env{Config: &config.Config{SecureCookieKey: "k"}, Srv: &services.Services{Pulses: pulses}}}
	sig := websub.Signature(900, "k")

	rec := hubCall(h, h.Verify, http.MethodGet, sig, "hub.mode=subscribe&hub.challenge=abc123&hub.lease_seconds=3600")
	if rec.Code != http.StatusOK || rec.Body.String() != "abc123" || pulses.verified == nil || time.Until(*pulses.verified) > 2*time.Hour {
		t.Errorf("checked: %d %q, until %v", rec.Code, rec.Body.String(), pulses.verified)
	}

	if rec := hubCall(h, h.Notify, http.MethodPost, sig, ""); rec.Code != http.StatusAccepted || pulses.soon != 1 {
		t.Errorf("told: %d, read soon %d", rec.Code, pulses.soon)
	}

	// Without the signature, or with no hub asked, it is not there.
	if rec := hubCall(h, h.Notify, http.MethodPost, "wrong", ""); rec.Code != http.StatusNotFound || pulses.soon != 1 {
		t.Errorf("unsigned: %d", rec.Code)
	}
	pulses.hub = ""
	if rec := hubCall(h, h.Verify, http.MethodGet, sig, "hub.mode=subscribe&hub.challenge=x"); rec.Code != http.StatusNotFound {
		t.Errorf("no hub asked: %d", rec.Code)
	}
}
