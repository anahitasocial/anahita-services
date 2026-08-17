package constants

import "time"

const TotpIssuer = "anahita.io"
const TotpIssuerStaging = "anahita.io Staging"
const TotpNumRecoveryCodes = 16
const TotpRecoveryCodeLength = 8
const TotpRecoveryCodeSep = ","
const TotpExpiryTime = 60 * time.Minute
