package constants

const (
	// Scope errors
	ErrInsufficientScope = "insufficient scope"

	// Auth log permission errors
	ErrAuthLogBrowseNotAllowed = "you are not allowed to browse authentication logs"
	ErrAuthLogAddNotAllowed    = "not allowed to add authentication log"
	ErrAuthLogDeleteNotAllowed = "you are not allowed to delete this authentication log"

	// OAuth permission errors
	ErrOAuthClientBrowseNotAllowed = "you are not allowed to browse OAuth clients"
	ErrOAuthClientReadNotAllowed   = "you are not allowed to read this OAuth client"
	ErrOAuthClientEditNotAllowed   = "you are not allowed to edit this OAuth client"
	ErrOAuthClientAddNotAllowed    = "you are not allowed to add an OAuth client"
	ErrOAuthClientDeleteNotAllowed = "you are not allowed to delete this OAuth client"

	// User permission errors
	ErrUserBrowseNotAllowed       = "you are not allowed to browse users"
	ErrUserReadNotAllowed         = "you are not allowed to read this user"
	ErrUserEditNotAllowed         = "you are not allowed to edit this user"
	ErrUserAddNotAllowed          = "you are not allowed to add a user"
	ErrUserDeleteNotAllowed       = "you are not allowed to delete this user"
	ErrUserAdministrateNotAllowed = "you are not allowed to administrate this user"

	// TOTP permission errors
	ErrTotpNoUserPermissions  = "you do not have the right user permissions"
	ErrTotpTotpNotInitiated   = "totp isn't initiated"
	ErrTotpTotpAlreadyEnabled = "there is a TOTP already enabled. You must disable it first before enabling a new TOTP"
)
