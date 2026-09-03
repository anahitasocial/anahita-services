package constants

import "time"

// The issuer label lived here as TotpIssuer/TotpIssuerStaging, hardcoded
// to "anahita.io". It has moved to auth-service config as TOTP_ISSUER
// (see Config.GetTotpIssuer), because it is a per-deployment display
// value rather than a protocol constant — and because it is baked into
// the QR code, so a deployment that shipped the wrong one could not
// correct it without every person re-enrolling.
//
// The rest of this file is genuinely fixed: code length, count, and
// separator are format decisions, not deployment ones.

const TotpNumRecoveryCodes = 16
const TotpRecoveryCodeLength = 8
const TotpRecoveryCodeSep = ","
const TotpExpiryTime = 60 * time.Minute
