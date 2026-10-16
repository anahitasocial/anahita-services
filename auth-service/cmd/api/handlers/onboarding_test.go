package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth/data/entities"

	libconstants "anahita-libs/constants"
	libentities "anahita-libs/data/entities"
)

// PATCH /onboarding — recording that the viewer has been through it.

func onboardingViewer(id int64) *libentities.Viewer {
	v := testViewer(id, 7)
	v.PersonType = libconstants.PersonTypeRegistered
	return v
}

func decodeOnboarding(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	return decoded
}

func TestOnboardingEditRecordsTheViewer(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))

	c, rec := patchJSON(newTestEcho(), ``, onboardingViewer(42))
	if err := newOnboardingHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}
	answers.Handler(newOnboardingHandler(te.Env).Edit, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if te.Users.byID[42].OnboardedAt == nil {
		t.Fatal("onboarded_at was not set")
	}
	if got := decodeOnboarding(t, rec.Body.Bytes())["onboarded_at"]; got == nil {
		t.Errorf("onboarded_at = null in the response; it was just set")
	}
}

// A second call — two tabs, a retry — answers with the date already stored,
// not with now.
func TestOnboardingEditKeepsTheFirstDate(t *testing.T) {
	user := testUser(t, 42, "ada@example.com", "the current password value")
	first := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	user.OnboardedAt = &first
	te := newTestEnv(user)

	c, rec := patchJSON(newTestEcho(), ``, onboardingViewer(42))
	if err := newOnboardingHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}
	answers.Handler(newOnboardingHandler(te.Env).Edit, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := decodeOnboarding(t, rec.Body.Bytes())["onboarded_at"]; got != "2026-09-01T12:00:00Z" {
		t.Errorf("onboarded_at = %v, want the original date", got)
	}
}

// Nobody signed in, nobody to record. The token middleware normally stops this
// earlier; the handler does not rely on it.
func TestOnboardingEditRefusesGuests(t *testing.T) {
	for name, viewer := range map[string]*libentities.Viewer{
		"no viewer":  nil,
		"id zero":    onboardingViewer(0),
		"guest type": testViewer(42, 7),
	} {
		t.Run(name, func(t *testing.T) {
			te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))

			c, rec := patchJSON(newTestEcho(), ``, viewer)
			if err := newOnboardingHandler(te.Env).Edit(c); err != nil {
				t.Fatalf("edit: %v", err)
			}
			answers.Handler(newOnboardingHandler(te.Env).Edit, rec)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if te.Users.onboardCalls != 0 {
				t.Error("onboarding was recorded with nobody signed in")
			}
		})
	}
}

func TestOnboardingEditReportsAStoreFailure(t *testing.T) {
	te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))
	te.Users.onboardErr = errStoreDown

	c, rec := patchJSON(newTestEcho(), ``, onboardingViewer(42))
	if err := newOnboardingHandler(te.Env).Edit(c); err != nil {
		t.Fatalf("edit: %v", err)
	}
	answers.Handler(newOnboardingHandler(te.Env).Edit, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// GET /onboarding — who invited the viewer, by alias.

func readOnboarding(t *testing.T, te *testEnv, viewer *libentities.Viewer) *httptest.ResponseRecorder {
	t.Helper()
	c, rec := get(newTestEcho(), "/onboarding")
	if viewer != nil {
		c.Set(libconstants.ViewerContextKey, viewer)
	}
	if err := newOnboardingHandler(te.Env).Read(c); err != nil {
		t.Fatalf("read: %v", err)
	}
	answers.Handler(newOnboardingHandler(te.Env).Read, rec)
	return rec
}

func TestOnboardingReadNamesTheInviter(t *testing.T) {
	inviter := testUser(t, 7, "grace@example.com", "the inviter's password value")
	inviter.Alias = "grace"
	inviter.Enabled = true

	invited := testUser(t, 42, "ada@example.com", "the current password value")
	invited.InvitedBy = 7

	rec := readOnboarding(t, newTestEnv(inviter, invited), onboardingViewer(42))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if got := decodeOnboarding(t, rec.Body.Bytes())["invited_by"]; got != "grace" {
		t.Errorf("invited_by = %v, want \"grace\"", got)
	}
}

// null, present, for every way of having no inviter to show. Absent would read
// as an older server; null says there is nobody.
func TestOnboardingReadHasNoInviterToShow(t *testing.T) {
	deleted := time.Now()

	for name, inviter := range map[string]func(*entities.User){
		"nobody invited them":     nil,
		"the inviter is gone":     func(u *entities.User) { u.ID = 999 },
		"the inviter is disabled": func(u *entities.User) { u.Enabled = false },
		"the inviter is deleting": func(u *entities.User) { u.DeletedAt = &deleted },
	} {
		t.Run(name, func(t *testing.T) {
			grace := testUser(t, 7, "grace@example.com", "the inviter's password value")
			grace.Alias = "grace"
			grace.Enabled = true

			invited := testUser(t, 42, "ada@example.com", "the current password value")
			if inviter != nil {
				invited.InvitedBy = 7
				inviter(grace)
			}

			rec := readOnboarding(t, newTestEnv(grace, invited), onboardingViewer(42))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 — a missing inviter must not "+
					"take the step with it", rec.Code)
			}
			decoded := decodeOnboarding(t, rec.Body.Bytes())
			value, present := decoded["invited_by"]
			if !present {
				t.Fatal("invited_by is absent; it must be null")
			}
			if value != nil {
				t.Errorf("invited_by = %v, want null", value)
			}
		})
	}
}

func TestOnboardingReadRefusesGuests(t *testing.T) {
	for name, viewer := range map[string]*libentities.Viewer{
		"no viewer":  nil,
		"id zero":    onboardingViewer(0),
		"guest type": testViewer(42, 7),
	} {
		t.Run(name, func(t *testing.T) {
			te := newTestEnv(testUser(t, 42, "ada@example.com", "the current password value"))

			if rec := readOnboarding(t, te, viewer); rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}
