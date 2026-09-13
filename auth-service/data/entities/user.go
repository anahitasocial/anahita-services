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

type User struct {
	entities.AnahitaNode

	PersonType string `json:"usertype"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"-"`

	GivenName     string    `json:"given_name"`
	FamilyName    string    `json:"family_name"`
	LastVisitDate time.Time `json:"last_visit_date,omitempty"`

	// DeletedAt is set when the account is scheduled for deletion, and nil
	// otherwise — which is almost always.
	//
	// Deleting also clears Enabled, so every login path already refuses the
	// account without consulting this. What this adds is the ability to tell
	// the two apart: an account an administrator disabled should stay refused,
	// while one the person deleted themselves should be offered back during the
	// grace period. Without it both look identical and neither can be restored.
	DeletedAt *time.Time `json:"-"`

	// TOTP fields
	TotpSecret         string
	TotpRecoveryCodes  string
	TotpEnabled        bool
	TotpLastVerifiedAt time.Time
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
	if u.Username == "" {
		return errors.New("username is required")
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
