package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/data/entities"
	"auth/data/repositories"

	libs_entities "anahita-libs/data/entities"
)

// fakeRefreshTokens is a stand-in for the refresh token repository that
// records whether the reuse cascade fired.
//
// Only the revoked-token branch of RefreshTokenPair is exercised here, and
// it returns before reaching the user lookup or the signer, so nothing
// else on OAuthService needs constructing.
type fakeRefreshTokens struct {
	stored *entities.OAuthRefreshToken

	cascadeCalls   int
	cascadeUserID  int64
	cascadeClient  string
	revokeAllCalls int
}

func (f *fakeRefreshTokens) ReadByTokenHash(string) (*entities.OAuthRefreshToken, error) {
	return f.stored, nil
}

func (f *fakeRefreshTokens) RevokeByUserAndClient(userID int64, clientID string) error {
	f.cascadeCalls++
	f.cascadeUserID = userID
	f.cascadeClient = clientID
	return nil
}

func (f *fakeRefreshTokens) RevokeByClientID(string) error {
	f.revokeAllCalls++
	return nil
}

// Unused by this path.
func (f *fakeRefreshTokens) ReadByID(int64) (*entities.OAuthRefreshToken, error) {
	return nil, nil
}
func (f *fakeRefreshTokens) BrowseByViewer(*libs_entities.Viewer) ([]*entities.OAuthRefreshToken, error) {
	return nil, nil
}
func (f *fakeRefreshTokens) BrowseByClientID(string) ([]*entities.OAuthRefreshToken, error) {
	return nil, nil
}
func (f *fakeRefreshTokens) Add(t *entities.OAuthRefreshToken) (*entities.OAuthRefreshToken, error) {
	return t, nil
}
func (f *fakeRefreshTokens) Revoke(int64) error                         { return nil }
func (f *fakeRefreshTokens) RevokeIfActive(int64) (bool, error)         { return true, nil }
func (f *fakeRefreshTokens) RevokeByViewer(*libs_entities.Viewer) error { return nil }
func (f *fakeRefreshTokens) DeleteExpired() (int64, error)              { return 0, nil }

func serviceWithStoredToken(stored *entities.OAuthRefreshToken) (*OAuthService, *fakeRefreshTokens) {
	fake := &fakeRefreshTokens{stored: stored}
	return &OAuthService{
		Repos: &repositories.Repositories{OAuthRefreshTokens: fake},
	}, fake
}

func revokedToken(revokedAt *time.Time) *entities.OAuthRefreshToken {
	return &entities.OAuthRefreshToken{
		ID:        42,
		UserID:    7,
		ClientID:  "anahita-mobile",
		Scopes:    "openid offline_access",
		Revoked:   true,
		RevokedAt: revokedAt,
		ExpiresAt: time.Now().Add(RefreshTokenLifetime),
	}
}

func testClient() *entities.OAuthClient {
	return &entities.OAuthClient{ClientID: "anahita-mobile"}
}

// TestRefreshTokenPair_RotationRaceIsNotReuse is the regression test for the
// staging incident: a client that races itself (every pre-single-flight
// build does, once per sync cycle) presented a token rotated milliseconds
// earlier, which was read as a replay and revoked every token that user held
// for the client — signing them out of the same app on other devices.
func TestRefreshTokenPair_RotationRaceIsNotReuse(t *testing.T) {
	justNow := time.Now().Add(-5 * time.Second)
	svc, fake := serviceWithStoredToken(revokedToken(&justNow))

	_, err := svc.RefreshTokenPair(context.Background(), "plaintext", testClient(), "")

	if !errors.Is(err, entities.ErrRefreshTokenRaced) {
		t.Errorf("err = %v, want ErrRefreshTokenRaced", err)
	}
	if fake.cascadeCalls != 0 {
		t.Errorf("a rotation race must not revoke the user's other sessions, got %d cascades", fake.cascadeCalls)
	}
	if fake.revokeAllCalls != 0 {
		t.Errorf("and must never reach the client-wide hammer, got %d calls", fake.revokeAllCalls)
	}
}

// TestRefreshTokenPair_ReplayAfterGraceCascades confirms the grace window
// did not simply disable reuse detection.
func TestRefreshTokenPair_ReplayAfterGraceCascades(t *testing.T) {
	longAgo := time.Now().Add(-10 * time.Minute)
	svc, fake := serviceWithStoredToken(revokedToken(&longAgo))

	_, err := svc.RefreshTokenPair(context.Background(), "plaintext", testClient(), "")

	if !errors.Is(err, entities.ErrRefreshTokenReuse) {
		t.Errorf("err = %v, want ErrRefreshTokenReuse", err)
	}
	if fake.cascadeCalls != 1 {
		t.Fatalf("cascadeCalls = %d, want 1", fake.cascadeCalls)
	}
	if fake.cascadeUserID != 7 {
		t.Errorf("cascade must be scoped to the token's user: got user_id %d, want 7", fake.cascadeUserID)
	}
	if fake.cascadeClient != "anahita-mobile" {
		t.Errorf("cascadeClient = %q, want %q", fake.cascadeClient, "anahita-mobile")
	}
	if fake.revokeAllCalls != 0 {
		t.Errorf("never client-wide, got %d calls", fake.revokeAllCalls)
	}
}

// TestRefreshTokenPair_CascadeFollowsTheTokensOwnClient covers a replayed
// token presented by a different client than the one it was issued to.
//
// The reuse branch runs before the ownership check, and a public client
// needs no secret to authenticate, so the two genuinely can differ. The
// cascade has to follow the token's own client: scoping it to the
// presenting client would revoke an unrelated session family and leave the
// compromised one live.
func TestRefreshTokenPair_CascadeFollowsTheTokensOwnClient(t *testing.T) {
	longAgo := time.Now().Add(-10 * time.Minute)
	stored := revokedToken(&longAgo)
	stored.ClientID = "anahita-web"
	svc, fake := serviceWithStoredToken(stored)

	// Presented by anahita-mobile, a public client holding a web token.
	_, err := svc.RefreshTokenPair(context.Background(), "plaintext", testClient(), "")

	if !errors.Is(err, entities.ErrRefreshTokenReuse) {
		t.Errorf("err = %v, want ErrRefreshTokenReuse", err)
	}
	if fake.cascadeClient != "anahita-web" {
		t.Errorf("cascadeClient = %q, want %q (the token's own client, not the presenter)",
			fake.cascadeClient, "anahita-web")
	}
}

// TestRefreshTokenPair_NilRevokedAtCascades covers rows revoked before the
// revoked_at column existed. Unknown age must fail toward containment.
func TestRefreshTokenPair_NilRevokedAtCascades(t *testing.T) {
	svc, fake := serviceWithStoredToken(revokedToken(nil))

	_, err := svc.RefreshTokenPair(context.Background(), "plaintext", testClient(), "")

	if !errors.Is(err, entities.ErrRefreshTokenReuse) {
		t.Errorf("err = %v, want ErrRefreshTokenReuse", err)
	}
	if fake.cascadeCalls != 1 {
		t.Errorf("a pre-migration row has unknown age and must be treated as a replay, got %d cascades",
			fake.cascadeCalls)
	}
}

// TestRefreshTokenPair_GraceBoundary pins which side of ReuseGracePeriod
// each outcome falls on, so changing the constant cannot silently invert the
// behaviour.
func TestRefreshTokenPair_GraceBoundary(t *testing.T) {
	tests := []struct {
		name    string
		age     time.Duration
		wantErr error
		cascade int
	}{
		{"well inside the window", ReuseGracePeriod / 2, entities.ErrRefreshTokenRaced, 0},
		{"just inside the window", ReuseGracePeriod - time.Second, entities.ErrRefreshTokenRaced, 0},
		{"just outside the window", ReuseGracePeriod + time.Second, entities.ErrRefreshTokenReuse, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revokedAt := time.Now().Add(-tt.age)
			svc, fake := serviceWithStoredToken(revokedToken(&revokedAt))

			_, err := svc.RefreshTokenPair(context.Background(), "plaintext", testClient(), "")

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if fake.cascadeCalls != tt.cascade {
				t.Errorf("cascadeCalls = %d, want %d", fake.cascadeCalls, tt.cascade)
			}
		})
	}
}
