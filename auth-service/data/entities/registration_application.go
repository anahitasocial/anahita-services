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

const (
	// ApplicationStatusUnverified — the address has not been confirmed, so
	// the application is not in the queue yet. Administrators should not be
	// moderating entries nobody can even receive mail at.
	ApplicationStatusUnverified = "unverified"

	// ApplicationStatusPending — verified, waiting on a decision.
	ApplicationStatusPending = "pending"

	ApplicationStatusApproved = "approved"
	ApplicationStatusRejected = "rejected"
)

type RegistrationApplication struct {
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
	// person in the queue. Zero for a cold application.
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

func NewRegistrationApplication() *RegistrationApplication {
	return &RegistrationApplication{
		Status: ApplicationStatusUnverified,
	}
}

// SetPassword stores the hash the account will be created with.
//
// Hashed here rather than at approval, so the plaintext exists only for the
// length of the request that submitted it. An approval queue holding
// recoverable passwords would be a far worse thing to leak than a list of
// people who want to join.
func (a *RegistrationApplication) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	a.Password = string(hash)

	return nil
}

// IsEmailVerified reports whether the applicant has confirmed their address.
func (a *RegistrationApplication) IsEmailVerified() bool {
	return a.EmailVerifiedAt != nil
}

// IsDecided reports whether an administrator has already answered.
//
// Both outcomes count. A decided application is not re-decidable: approving a
// rejection would create an account somebody was refused, and rejecting an
// approval would leave an account behind with no application to explain it.
func (a *RegistrationApplication) IsDecided() bool {
	return a.Status == ApplicationStatusApproved ||
		a.Status == ApplicationStatusRejected
}

// Validate checks what the table cannot.
func (a *RegistrationApplication) Validate() error {
	if strings.TrimSpace(a.Username) == "" {
		return errors.New("username is required")
	}

	if strings.TrimSpace(a.Email) == "" {
		return errors.New("email is required")
	}

	// The hash, not the plaintext — SetPassword has already run by here, and
	// an application with no hash would approve into an account nobody can
	// sign into.
	if a.Password == "" {
		return errors.New("password is required")
	}

	return nil
}
