package handlers

import (
	"errors"
	"fmt"
	"log"
	"time"

	"auth/constants"
	"auth/data/entities"
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

// issueCode stores a new code for one person and returns its token, carrying
// the code to send. The code itself is not stored: only its keyed hash is.
func (env *Env) issueCode(user *entities.User, tokenType, channel string) (*entities.Token, error) {
	now := time.Now()

	token := entities.NewToken()
	token.Status = constants.TokenStatusPending
	token.Type = tokenType
	token.Channel = channel
	token.ExpiredAfter = constants.TokenExpiryTimeOTP
	token.RecipientEmail = user.Email
	token.IsUsed = false
	token.CreatedAt = now
	token.CreatedBy = user.ID
	token.UpdatedAt = now
	token.UpdatedBy = user.ID

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
