package entities

import (
	"anahita-libs/constants"
	"anahita-libs/crypt"
	"anahita-libs/data/entities"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User is a person node plus the account that signs in as it.
//
// Only the credential half is declared here. PersonType, Username, GivenName,
// FamilyName, LastVisitDate and DeletedAt are all gone: each was a copy of
// something the embedded node already carries, and an outer field of the same
// name SHADOWS the embedded one — so the node's value was loaded and then read
// from the wrong place. Person had the identical problem and the identical fix.
//
//	Name       the display name, where GivenName and FamilyName used to be
//	Alias      the handle, where Username used to be
//	PersonType the authority level
//
// What remains comes from auth_accounts, and none of it belongs on a node that
// sixteen callers can read.
type User struct {
	entities.AnahitaNode

	Email    string `json:"email"`
	Password string `json:"-"`

	// EmailVerifiedAt is when control of the address was proved, and nil when
	// it has not been.
	//
	// A pointer, so "never" is expressible. A zero time would be both a
	// fabricated date and, to MySQL under a strict sql_mode, '0000-00-00'.
	EmailVerifiedAt *time.Time `json:"-"`

	// OnboardedAt is when the person went through onboarding, and nil when
	// they have not. Completing the steps and skipping them both count — it
	// records that the walk-through was offered, not what came of it.
	OnboardedAt *time.Time `json:"-"`

	// HasBio reports whether the node's body holds anything but whitespace.
	//
	// A flag rather than the text. The onboarding gate asks "is the profile
	// incomplete?", and answering it does not need the bio itself carried
	// through every sign-in and credential check that reads a User.
	HasBio bool `json:"-"`

	// InvitedBy is the actor whose invitation brought this person in, and 0
	// when nobody's did. Written when the account is created and never after.
	InvitedBy int64 `json:"-"`

	// TOTP fields
	TotpSecret         string
	TotpRecoveryCodes  string
	TotpEnabled        bool
	TotpLastVerifiedAt time.Time

	// What this account agreed to, and when. Captured at account creation —
	// an account should not exist without it — and again whenever the version
	// in force moves on. See consent.go.
	//
	// Written in full; only Accepted and Version are read back. The audit
	// trio is evidence for a person reading the row, not input to any
	// decision the code makes, and carrying three more strings through every
	// sign-in to never look at them is not worth the column reads.
	TermsOfService Consent `json:"-"`
	PrivacyPolicy  Consent `json:"-"`
}

// NewUser creates a new user
func NewUser() *User {
	return &User{
		AnahitaNode: *entities.NewAnahitaNode(),
	}
}

// SetPassword sets the Password field with an encrypted password
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u.Password = string(hash)

	return nil
}

// CheckPassword checks if the password matches the encrypted password
func (u *User) CheckPassword(password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return false
	}

	return true
}

// Validate validates the user
func (u *User) Validate() error {
	// The handle is nodes.alias, validated by the graph when the actor is
	// created. Checking it here would be checking a copy.
	if u.Alias == "" {
		return errors.New("alias is required")
	}

	if u.Password == "" {
		return errors.New("password is required")
	}

	if u.Email == "" {
		return errors.New("email is required")
	}

	if u.PersonType == "" {
		return errors.New("user type is required")
	}

	return nil
}

// IsEmailVerified reports whether this account has proved control of its
// address.
//
// An unverified account cannot sign in. Anything softer is a verification step
// people learn to ignore — and the address is what password reset delivers to,
// so an unverified one is a reset link pointed at somebody else's inbox.
func (u *User) IsEmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

// MarkEmailVerified stamps the address as confirmed, now.
func (u *User) MarkEmailVerified() {
	now := time.Now()
	u.EmailVerifiedAt = &now
}

func (u *User) IsGuest() bool {
	return u.PersonType == constants.PersonTypeGuest
}

func (u *User) IsRegistered() bool {
	return u.PersonType == constants.PersonTypeRegistered
}

func (u *User) IsAdmin() bool {
	return u.PersonType == constants.PersonTypeAdmin
}

func (u *User) IsSuperAdmin() bool {
	return u.PersonType == constants.PersonTypeSuperAdmin
}

func (u *User) IsAdminOrSuperAdmin() bool {
	return u.IsAdmin() || u.IsSuperAdmin()
}

// SetTotpSecret encrypts and sets the TOTP secret for the user
func (u *User) SetTotpSecret(crypt crypt.Crypt, totpSecret string) error {
	result, err := crypt.Encrypt(totpSecret)
	if err != nil {
		return err
	}

	u.TotpSecret = result

	return nil
}

// GetTotpSecret decrypts and returns the TOTP secret for the user
func (u *User) GetTotpSecret(crypt crypt.Crypt) (string, error) {
	if u.TotpSecret == "" {
		return "", errors.New("error obtaining TOTP secret")
	}

	result, err := crypt.Decrypt(u.TotpSecret)
	if err != nil {
		return "", err
	}

	return result, nil
}

// Sets TOTP Recovery codes to the user and encrypts them using bcrypt hash algorithm
func (u *User) SetRecoveryCodes(codes []string) error {
	hashed := make([]string, 0)
	for i := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(codes[i]), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		hashed = append(hashed, string(hash))
	}

	if len(hashed) > 0 {
		u.TotpRecoveryCodes = strings.Join(hashed, constants.TotpRecoveryCodeSep)
	}

	return nil
}

// Validates a Recovery code, if true it removes it from the list.
// User has to be updated immediately after using this method
func (u *User) ValidateRecoveryCode(code string) bool {
	rCodes := strings.Split(u.TotpRecoveryCodes, constants.TotpRecoveryCodeSep)

	for i := range rCodes {
		if err := bcrypt.CompareHashAndPassword([]byte(rCodes[i]), []byte(code)); err == nil {
			newCodes := append(rCodes[:i], rCodes[i+1:]...)
			u.TotpRecoveryCodes = strings.Join(newCodes, constants.TotpRecoveryCodeSep)

			return true
		}
	}

	return false
}

// CountRecoveryCodes reports how many unused recovery codes remain.
//
// Splitting an empty string yields one empty element rather than none, so the
// naive len(Split(...)) reads 1 for an account with no codes at all — which is
// exactly the number that would be reported to somebody who has just spent
// their last one, and exactly the person who needs to be told otherwise.
func (u *User) CountRecoveryCodes() int {
	if u.TotpRecoveryCodes == "" {
		return 0
	}

	count := 0
	for _, code := range strings.Split(u.TotpRecoveryCodes, constants.TotpRecoveryCodeSep) {
		if strings.TrimSpace(code) != "" {
			count++
		}
	}

	return count
}

// IsMfaExpired checks if the user's MFA is expired
func (u *User) IsMfaExpired() bool {
	return u.TotpEnabled && u.TotpLastVerifiedAt.Add(constants.TotpExpiryTime).Before(time.Now())
}
