package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"auth/constants"
	"auth/data/entities"

	"github.com/labstack/echo/v4"
)

// One-time codes: the six digits a person is sent and types back.
//
// Shared by every flow that uses one. Each flow has its own token type, and a
// code is only ever checked against the type it was issued for, so a code sent
// to sign in can never be used for anything else.
//
// Two rules hold everywhere and are easy to break:
//
//   - A code is found by its token's ID, never by the code. The ID lives in
//     the browser's server-side session. Looking up "the token whose code is
//     123456" would match anybody's live code.
//   - Counting an attempt and using a code up are each one statement in the
//     database. Submissions arriving together cannot share an attempt, and
//     only one of two correct ones succeeds.

var (
	// errCodeGone covers every reason a code can no longer be used: never
	// issued, of another type, already used, expired, or out of attempts.
	// One error, because the person is told one thing: ask for a new one.
	errCodeGone = errors.New("code: no usable code")

	// errCodeMismatch is a wrong code with attempts still left.
	errCodeMismatch = errors.New("code: does not match")
)

// codeRequest says who a code is for and what it may be used for.
type codeRequest struct {
	// Email is where the code is sent, and how the account is found again
	// when the code comes back.
	Email string

	// PersonID is the account, or zero for somebody applying to join, who
	// has none yet.
	PersonID int64

	Type    string
	Channel string

	// Lifetime in seconds.
	Lifetime int

	// NewEmail is the address being moved to, on an email change only.
	NewEmail string
}

// issueCode retires the earlier codes of this type for this address, stores a
// new one, and returns its token, carrying the code to send. The code itself
// is not stored: only its keyed hash is.
//
// Retiring first means there is never more than one live code per purpose
// and address, each with its own five attempts.
func (env *Env) issueCode(req codeRequest) (*entities.Token, error) {
	if _, err := env.Repos.Tokens.InvalidateOutstanding(req.Type, req.Email); err != nil {
		return nil, fmt.Errorf("issue code: retiring earlier codes: %w", err)
	}

	now := time.Now()

	token := entities.NewToken()
	token.Status = constants.TokenStatusPending
	token.Type = req.Type
	token.Channel = req.Channel
	token.ExpiredAfter = req.Lifetime
	token.RecipientEmail = req.Email
	token.NewEmail = req.NewEmail
	token.IsUsed = false
	token.CreatedAt = now
	token.CreatedBy = req.PersonID
	token.UpdatedAt = now
	token.UpdatedBy = req.PersonID

	// The body is the row's identifier and the hash's salt. It is never sent.
	if err := token.GenerateToken(); err != nil {
		return nil, fmt.Errorf("issue code: %w", err)
	}

	if err := token.GenerateCode(env.Config.GetCodeKey()); err != nil {
		return nil, fmt.Errorf("issue code: %w", err)
	}

	stored, err := env.Repos.Tokens.Add(token)
	if err != nil {
		return nil, fmt.Errorf("issue code: %w", err)
	}

	if stored.Code == "" {
		return nil, errors.New("issue code: the stored token carries no code to send")
	}

	return stored, nil
}

// verifyCode checks a submission against one token. On success the token is
// returned, NOT yet used up: the caller checks the account first, then calls
// consumeCode.
//
// Every submission spends an attempt, the correct one included, and it is
// spent before the comparison. Spending only on a mismatch would need a read,
// a compare and a write, and guesses arriving together would slip between
// them.
//
// The second value is how many attempts are left after this one.
func (env *Env) verifyCode(tokenID int64, tokenType, submitted string) (*entities.Token, int, error) {
	if tokenID == 0 {
		return nil, 0, errCodeGone
	}

	token, err := env.Repos.Tokens.ReadByID(tokenID)
	if err != nil || token == nil {
		log.Println("code: token read failed:", err)
		return nil, 0, errCodeGone
	}

	if token.Type != tokenType || token.IsUsed || token.IsExpired() {
		return nil, 0, errCodeGone
	}

	spent, err := env.Repos.Tokens.SpendCodeAttempt(token.ID)
	if err != nil {
		// Closed, not open: a code that cannot be counted is not checked.
		log.Println("code: could not count the attempt:", err)
		return nil, 0, errCodeGone
	}
	if !spent {
		return nil, 0, errCodeGone
	}

	remaining := constants.OTPMaxAttempts - (token.CodeAttempts + 1)
	if remaining < 0 {
		remaining = 0
	}

	if !token.IsCodeValid(env.Config.GetCodeKey(), submitted) {
		if remaining == 0 {
			return nil, 0, errCodeGone
		}
		return nil, remaining, errCodeMismatch
	}

	return token, remaining, nil
}

// consumeCode uses the code up, and reports whether this call was the one
// that did. False means another submission got there first.
func (env *Env) consumeCode(token *entities.Token, personID int64) bool {
	consumed, err := env.Repos.Tokens.ConsumeCode(token.ID, personID)
	if err != nil {
		log.Println("code: could not mark the code used:", err)
		return false
	}
	return consumed
}

// ---------------------------------------------------------------------
// A browser waiting for a code
// ---------------------------------------------------------------------

// codeStep is one browser's place in a flow that has sent a code and is
// waiting for it to be typed. It is held in the server-side session.
type codeStep struct {
	// Subject is what the person typed to start the flow, shown back to
	// them. Never the account's address when they typed something else.
	Subject string `json:"subject"`

	// TokenID is the code's row, or zero when nothing was sent. A step
	// with no token looks the same from outside and runs out after the
	// same five tries, which is what keeps these pages from saying whether
	// an account exists.
	TokenID int64 `json:"token_id"`

	ExpiresAt time.Time `json:"expires_at"`

	// Attempts counts tries when there is no token to count them on.
	Attempts int `json:"attempts"`

	// Verified is set once the code has been accepted, by the flows that
	// have a further step: choosing a new password comes after the code.
	Verified bool `json:"verified,omitempty"`
}

type codeResult int

const (
	// codeAccepted: the code is right. The token is returned, not yet used.
	codeAccepted codeResult = iota
	// codeMalformed: not six digits. Nothing was spent.
	codeMalformed
	// codeWrong: a wrong code, with tries left.
	codeWrong
	// codeDead: expired, used, or out of tries. The step has been removed.
	codeDead
)

func (env *Env) writeCodeStep(c echo.Context, key string, step codeStep, ttl time.Duration) error {
	raw, err := json.Marshal(step)
	if err != nil {
		return err
	}

	// In place when the browser already has it. Add alone would hand out a
	// new session on every step and leave the earlier ones behind.
	if existing, err := env.Session.Read(c, key); err == nil && existing != "" {
		if err := env.Session.Edit(c, key, string(raw)); err == nil {
			return nil
		}
	}

	_, err = env.Session.AddWithTTL(c, key, string(raw), ttl)
	return err
}

func (env *Env) readCodeStep(c echo.Context, key string) (*codeStep, error) {
	raw, err := env.Session.Read(c, key)
	if err != nil || raw == "" {
		return nil, err
	}

	var step codeStep
	if err := json.Unmarshal([]byte(raw), &step); err != nil {
		return nil, err
	}

	if time.Now().After(step.ExpiresAt) {
		_ = env.Session.Delete(c, key)
		return nil, nil
	}

	return &step, nil
}

// submitCode checks what was typed against the step's code. The third value
// is how many tries are left, when that is worth saying.
func (env *Env) submitCode(
	c echo.Context, key string, step *codeStep, tokenType, typed string,
) (*entities.Token, codeResult, int) {
	// Something that is not six digits cannot be a code, so it costs
	// nothing. A slip of the finger should not use up a try.
	typed = strings.TrimSpace(typed)
	if len(typed) != constants.OTPCodeLength || !isAllDigits(typed) {
		return nil, codeMalformed, 0
	}

	// Nothing was sent. Count the guess on the step, so it runs out after
	// the same five tries it would for a real account.
	if step.TokenID == 0 {
		step.Attempts++
		if step.Attempts >= constants.OTPMaxAttempts {
			_ = env.Session.Delete(c, key)
			return nil, codeDead, 0
		}
		if err := env.writeCodeStep(c, key, *step, time.Until(step.ExpiresAt)); err != nil {
			log.Println("code: could not record the attempt:", err)
		}
		return nil, codeWrong, constants.OTPMaxAttempts - step.Attempts
	}

	token, remaining, err := env.verifyCode(step.TokenID, tokenType, typed)
	if err != nil {
		if errors.Is(err, errCodeMismatch) {
			return nil, codeWrong, remaining
		}
		_ = env.Session.Delete(c, key)
		return nil, codeDead, 0
	}

	return token, codeAccepted, remaining
}
