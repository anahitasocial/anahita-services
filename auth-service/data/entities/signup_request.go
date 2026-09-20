package entities

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Somebody who has asked to join and is waiting on a decision.
//
// Not an account. There is no node and no auth_accounts row behind this until
// an administrator approves it — see the migration for why that matters once
// this instance federates.
//
// The same shape as a follow request, deliberately: one party asks, another
// answers, and nothing exists in between but the asking.

const (
	// SignupRequestStatusUnverified — the address has not been confirmed, so
	// the request is not in the queue yet. Administrators should not be
	// moderating entries nobody can even receive mail at.
	SignupRequestStatusUnverified = "unverified"

	// SignupRequestStatusPending — verified, waiting on a decision.
	SignupRequestStatusPending = "pending"

	SignupRequestStatusApproved = "approved"
	SignupRequestStatusRejected = "rejected"
)

type SignupRequest struct {
	ID       int64
	Username string
	Email    string
	Password string

	Name       string
	Body       string
	WebsiteURL string

	Status          string
	EmailVerifiedAt *time.Time

	// InvitedBy is the member who vouched, when a member's invitation put this
	// person in the queue. Zero for a cold request.
	//
	// An administrator's invitation never reaches the queue at all — it is
	// already the decision an approver would be making.
	InvitedBy int64

	DecidedBy    int64
	DecidedAt    *time.Time
	DecisionNote string

	CreatedIP        string
	CreatedUserAgent string

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewSignupRequest() *SignupRequest {
	return &SignupRequest{
		Status: SignupRequestStatusUnverified,
	}
}

// SetPassword stores the hash the account will be created with.
//
// Hashed here rather than at approval, so the plaintext exists only for the
// length of the request that submitted it. A signup queue holding recoverable
// passwords would be a far worse thing to leak than a list of people who want
// to join.
func (s *SignupRequest) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	s.Password = string(hash)

	return nil
}

// IsEmailVerified reports whether they have confirmed their address.
func (s *SignupRequest) IsEmailVerified() bool {
	return s.EmailVerifiedAt != nil
}

// IsDecided reports whether an administrator has already answered.
//
// Both outcomes count. A decided request is not re-decidable: approving a
// rejection would create an account somebody was refused, and rejecting an
// approval would leave an account behind with nothing to explain it.
func (s *SignupRequest) IsDecided() bool {
	return s.Status == SignupRequestStatusApproved ||
		s.Status == SignupRequestStatusRejected
}

// Validate checks what the table cannot.
func (s *SignupRequest) Validate() error {
	if strings.TrimSpace(s.Username) == "" {
		return errors.New("username is required")
	}

	if strings.TrimSpace(s.Email) == "" {
		return errors.New("email is required")
	}

	// The hash, not the plaintext — SetPassword has already run by here, and a
	// request with no hash would approve into an account nobody can sign into.
	if s.Password == "" {
		return errors.New("password is required")
	}

	return nil
}
