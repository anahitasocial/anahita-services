package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/data/entities"
	"auth/data/repositories"
)

// ctxSigner signs as a KMS signer does: not once its context is done.
type ctxSigner struct{ signed int }

func (s *ctxSigner) Algorithm() string { return "ES256" }
func (s *ctxSigner) ActiveKeyID(ctx context.Context) (string, error) {
	return "test-kid", ctx.Err()
}
func (s *ctxSigner) Sign(ctx context.Context, _ []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.signed++
	return []byte("signature"), nil
}

// Only ReadByID is called on a refresh; the rest of the interface is left
// to panic if it ever is.
type oneUser struct {
	repositories.UsersInterface
	user *entities.User
}

func (u oneUser) ReadByID(int64) (*entities.User, error) { return u.user, nil }

// No auth log follows the token here.
type noAuthLogs struct{ repositories.AuthLogsInterface }

func (noAuthLogs) ReadByOAuthRefreshTokenID(int64) (*entities.AuthLog, error) {
	return nil, errors.New("none")
}

// A request cancelled after the old refresh token was claimed must still get
// its successor: the old one is spent, and without a new one the client's
// retries end in ErrRefreshTokenReuse, which signs the person out of that
// client on every device.
func TestRefreshTokenPair_ACancelledRequestStillRotates(t *testing.T) {
	user := entities.NewUser()
	user.ID = 7
	user.Enabled = true
	active := &entities.OAuthRefreshToken{
		ID:        42,
		UserID:    7,
		ClientID:  "anahita-mobile",
		Scopes:    "offline_access",
		ExpiresAt: time.Now().Add(RefreshTokenLifetime),
	}
	signer := &ctxSigner{}
	svc := &OAuthService{
		Repos: &repositories.Repositories{
			OAuthRefreshTokens: &fakeRefreshTokens{stored: active},
			Users:              oneUser{user: user},
			AuthLogs:           noAuthLogs{},
		},
		signer: signer,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pair, err := svc.RefreshTokenPair(ctx, "plaintext", testClient(), "")
	if err != nil {
		t.Fatalf("a cancelled request lost the rotation: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Errorf("want a new pair, got %+v", pair)
	}
	if signer.signed != 1 {
		t.Errorf("signed %d times, want 1", signer.signed)
	}
}

// The rotation's context outlives its request's cancellation, keeps its
// values, and still has a limit of its own.
func TestRotationContext(t *testing.T) {
	type key struct{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "kept"))
	rotation, stop := rotationContext(parent)
	defer stop()
	cancel()

	if err := rotation.Err(); err != nil {
		t.Errorf("cancelled with its request: %v", err)
	}
	if rotation.Value(key{}) != "kept" {
		t.Error("lost the request's values")
	}
	deadline, ok := rotation.Deadline()
	if !ok || time.Until(deadline) > RotationTimeout {
		t.Errorf("deadline %v, %v; want one within %v", deadline, ok, RotationTimeout)
	}
}
