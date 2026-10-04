package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"auth/constants"
	"auth/data/entities"
	"auth/messages"
	"auth/requests"
	"auth/responses"

	"github.com/labstack/echo/v4"
	"github.com/mileusna/useragent"
)

// IPLocation is the location of the IP address
type IPLocation struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

// AuthLogHandler is the handler for authentication logs
type authLogHandler struct {
	*Env
}

// newAuthLogHandler returns a new auth log handler
func newAuthLogHandler(env *Env) *authLogHandler {
	return &authLogHandler{env}
}

// Browse returns a list of auth logs for a person.
func (h *authLogHandler) Browse(c echo.Context) error {
	var (
		viewer = getViewer(c)
		req    = requests.NewAuthLogRequest().Browse
		err    error
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err = h.Perms.AuthLog.CanBrowse(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	authLogs, err := h.Repos.AuthLogs.Browse(user)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	now := time.Now()
	for i := range authLogs {
		if authLogs[i].OAuthRefreshTokenID == 0 {
			continue
		}

		rt, err := h.Repos.OAuthRefreshTokens.ReadByID(authLogs[i].OAuthRefreshTokenID)
		if err != nil {
			continue
		}

		if !rt.Revoked && rt.ExpiresAt.After(now) {
			authLogs[i].IsActive = true
		}

		if viewer.OAuthRefreshTokenID != 0 && viewer.OAuthRefreshTokenID == authLogs[i].OAuthRefreshTokenID {
			authLogs[i].IsViewer = true
		}
	}

	respAuthLogs := &responses.AuthLogResponse{}
	resp := respAuthLogs.MapAuthLogItems(authLogs)

	return c.JSON(http.StatusOK, resp)
}

// Add records a new auth log entry on behalf of an explicit external API caller.
func (h *authLogHandler) Add(c echo.Context, refreshTokenID int64, clientID string, user *entities.User) (*entities.AuthLog, error) {
	if err := h.Perms.AuthLog.CanAdd(user); err != nil {
		return nil, err
	}
	return h.buildAndPersist(c, refreshTokenID, clientID, user)
}

// AddSystem records an auth log entry as a side-effect of token issuance.
func (h *authLogHandler) AddSystem(c echo.Context, refreshTokenID int64, clientID string, user *entities.User) (*entities.AuthLog, error) {
	return h.buildAndPersist(c, refreshTokenID, clientID, user)
}

// buildAndPersist constructs and persists the auth log row.
//
// Agent identification strategy:
//
//  1. Native clients (Electron, mobile, etc.) send X-Client-OS and
//     X-Client-App headers explicitly because their User-Agent strings
//     don't follow standard browser formats and useragent.Parse() can't
//     extract OS info reliably from them. When these headers are
//     present, we use them directly and skip UA parsing for OS/Name.
//
//  2. Browsers send conventional User-Agent strings that useragent.Parse()
//     handles correctly. When X-Client-* headers are absent, we parse
//     the User-Agent and read OS / Name from the result.
//
//  3. As a last resort (unknown UA, no client headers) we fall back
//     to the raw UA string as the agent name and "Unknown" for OS.
//
// Device classification stays UA-driven for browsers; native clients
// fall through to the desktop default.
func (h *authLogHandler) buildAndPersist(c echo.Context, refreshTokenID int64, clientID string, user *entities.User) (*entities.AuthLog, error) {
	authLog := h.buildAuthLog(c, refreshTokenID, clientID, user)

	persisted, err := h.Repos.AuthLogs.Add(authLog)
	if err != nil {
		return nil, err
	}

	if err := h.Repos.AuthLogs.Trim(user); err != nil {
		return nil, err
	}

	return persisted, nil
}

// AddOrReuseSystem records token issuance against an existing session
// row when this is the same browser re-authenticating silently, and
// only inserts a new row when it is genuinely a new session.
//
// The second return value reports whether an existing row was reused.
//
// Why this exists: the authorization_code branch runs every time the
// SPA's 15-minute access token expires and it bounces through
// /oauth/authorize, because the person_session cookie mints a fresh
// code without any credential event. Inserting unconditionally there
// turns one browser into a new "session" every 15 minutes, and Trim
// then evicts the user's genuinely distinct sessions within hours.
//
// A reused row keeps its original created_at: "signed in at 06:14" is
// the useful fact, not the timestamp of the latest token rotation.
//
// The superseded refresh token is revoked rather than left to expire.
// Once the row's pointer moves, no auth_log references the old token,
// which would put it beyond the reach of both DeleteAllSessions and
// the force-logout path — a live token that "sign out everywhere"
// cannot touch. Revoking costs nothing here: the client has already
// been handed its replacement pair and has no use for the old one.
func (h *authLogHandler) AddOrReuseSystem(c echo.Context, refreshTokenID int64, clientID string, user *entities.User) (*entities.AuthLog, bool, error) {
	authLog := h.buildAuthLog(c, refreshTokenID, clientID, user)

	existing, err := h.Repos.AuthLogs.ReadLatestByAgent(
		user.ID,
		clientID,
		authLog.Device,
		authLog.OS,
		authLog.Name,
		authLog.IP,
	)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("auth log: agent lookup failed, inserting new row:", err)
		}
		persisted, addErr := h.buildAndPersistFrom(authLog, user)
		return persisted, false, addErr
	}

	// An existing row whose token is already revoked or expired means
	// that session ended. This is a real new sign-in, not a rotation,
	// and deserves its own row and its own created_at.
	if !h.isSessionLive(existing) {
		persisted, addErr := h.buildAndPersistFrom(authLog, user)
		return persisted, false, addErr
	}

	if existing.OAuthRefreshTokenID != refreshTokenID {
		if err := h.Repos.OAuthRefreshTokens.Revoke(existing.OAuthRefreshTokenID); err != nil {
			// Non-fatal: the pointer move below is what keeps the
			// sessions list honest. A token we failed to revoke still
			// expires on its own.
			log.Println("auth log: revoking superseded refresh token failed:", err)
		}
	}

	if err := h.Repos.AuthLogs.UpdateOAuthRefreshTokenID(existing.ID, refreshTokenID); err != nil {
		return nil, false, err
	}

	existing.OAuthRefreshTokenID = refreshTokenID

	return existing, true, nil
}

// isSessionLive reports whether an auth log's current refresh token is
// still usable. A missing token row reads as not live.
func (h *authLogHandler) isSessionLive(authLog *entities.AuthLog) bool {
	if authLog.OAuthRefreshTokenID == 0 {
		return false
	}

	rt, err := h.Repos.OAuthRefreshTokens.ReadByID(authLog.OAuthRefreshTokenID)
	if err != nil {
		return false
	}

	return !rt.Revoked && rt.ExpiresAt.After(time.Now())
}

// buildAndPersistFrom inserts an already-built auth log and trims.
func (h *authLogHandler) buildAndPersistFrom(authLog *entities.AuthLog, user *entities.User) (*entities.AuthLog, error) {
	persisted, err := h.Repos.AuthLogs.Add(authLog)
	if err != nil {
		return nil, err
	}

	if err := h.Repos.AuthLogs.Trim(user); err != nil {
		return nil, err
	}

	return persisted, nil
}

// buildAuthLog constructs the auth log row without persisting it.
func (h *authLogHandler) buildAuthLog(c echo.Context, refreshTokenID int64, clientID string, user *entities.User) *entities.AuthLog {
	userAgentString := c.Request().Header.Get("User-Agent")
	clientOS := c.Request().Header.Get("X-Client-OS")
	clientApp := c.Request().Header.Get("X-Client-App")

	ua := useragent.Parse(userAgentString)

	authLog := entities.NewAuthLog()
	authLog.CreatedAt = time.Now()
	authLog.CreatedBy = user.ID
	authLog.IP = c.RealIP()
	authLog.City = messages.AuthLoginUnknownCity
	authLog.Country = messages.AuthLoginUnknownCountry
	authLog.OAuthRefreshTokenID = refreshTokenID
	authLog.ClientID = clientID

	// Agent name: prefer explicit X-Client-App, then UA-parsed name,
	// then raw UA string, then a placeholder.
	switch {
	case clientApp != "":
		authLog.Name = clientApp
	case ua.Name != "":
		authLog.Name = ua.Name
	case userAgentString != "":
		authLog.Name = userAgentString
	default:
		authLog.Name = constants.AuthLogAgentUnknown
	}

	// OS: prefer explicit X-Client-OS, then UA-parsed OS, then placeholder.
	switch {
	case clientOS != "":
		authLog.OS = clientOS
	case ua.OS != "":
		authLog.OS = ua.OS
	default:
		authLog.OS = constants.AuthLogOSUnknown
	}

	// Device: UA-driven where possible. Native clients without a
	// recognizable UA fall through to desktop — most native apps that
	// hit our auth endpoints run on desktop platforms.
	switch {
	case ua.Mobile:
		authLog.Device = constants.AuthLogDeviceMobile
	case ua.Tablet:
		authLog.Device = constants.AuthLogDeviceTablet
	case ua.Desktop:
		authLog.Device = constants.AuthLogDeviceDesktop
	case ua.Bot:
		authLog.Device = constants.AuthLogDeviceBot
	default:
		authLog.Device = constants.AuthLogDeviceDesktop
	}

	ipLoc := getIPLocation(authLog.IP)
	if ipLoc.City != "" && ipLoc.Country != "" {
		authLog.City = ipLoc.City
		authLog.Country = ipLoc.Country
	}

	return authLog
}

// Delete force-logs-out a session by revoking its refresh token.
func (h *authLogHandler) Delete(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewAuthLogRequest().Delete
	)

	if err = c.Bind(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	if err = c.Validate(req); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusBadRequest)
	}

	user, err := h.Repos.Users.ReadByID(req.UserID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if err := h.Perms.AuthLog.CanDelete(viewer, user); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusForbidden)
	}

	authLog, err := h.Repos.AuthLogs.ReadByID(req.ID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	// CanDelete authorized the viewer against the user in the path, not
	// against this row. Without this check a person passes their own uid
	// (clearing the permission check) plus any authLog id and force-logs
	// out a session belonging to someone else.
	//
	// 404 rather than 403: whether a given authLog id exists is not
	// something a non-owner should be able to probe.
	if authLog.CreatedBy != user.ID {
		log.Println("auth log delete: row", authLog.ID, "does not belong to user", user.ID)
		return c.NoContent(http.StatusNotFound)
	}

	if authLog.OAuthRefreshTokenID != 0 {
		if err := h.Repos.OAuthRefreshTokens.Revoke(authLog.OAuthRefreshTokenID); err != nil {
			log.Println("auth log delete: revoke failed, continuing:", err)
		}
	}

	if err := h.Repos.AuthLogs.Delete(authLog.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	// Only when somebody ELSE ended the session. A person clearing their own
	// is looking at the screen where they did it, and mailing them about it is
	// the kind of noise that teaches people to filter these messages — which
	// costs the ones that matter.
	//
	// The administrative case is the opposite: the owner is signed out of a
	// device mid-use with nothing anywhere to explain why.
	if viewer.ID != user.ID {
		notifyUser := *user
		device := describeDevice(authLog)
		location := describeLocation(authLog)
		go func() {
			if err := h.Notif.Auth.EmailSessionRevoked(&notifyUser, device, location); err != nil {
				log.Println("auth log: session revocation notification failed:", err)
			}
		}()
	}

	return c.NoContent(http.StatusOK)
}

// describeDevice renders an auth log row as something a person would recognise
// — "Firefox on a laptop running Linux".
//
// Falls back rather than rendering gaps: a row missing its user-agent detail
// would otherwise produce "on a running", which reads as a bug and undermines
// the message it appears in.
func describeDevice(authLog *entities.AuthLog) string {
	switch {
	case authLog.Name != "" && authLog.OS != "":
		return fmt.Sprintf("%s on %s", authLog.Name, authLog.OS)
	case authLog.Name != "":
		return authLog.Name
	case authLog.OS != "":
		return authLog.OS
	default:
		return "an unrecognised device"
	}
}

// describeLocation renders the geo columns, reusing the same fallbacks the
// login notification uses so the two messages describe a session identically.
func describeLocation(authLog *entities.AuthLog) string {
	city, country := authLog.City, authLog.Country
	if city == "" {
		city = messages.AuthLoginUnknownCity
	}
	if country == "" {
		country = messages.AuthLoginUnknownCountry
	}

	return fmt.Sprintf("%s, %s", city, country)
}

// DeleteAllSessions revokes every active refresh token a person holds.
//
// Called on password reset and TOTP reset. Both are recovery actions
// for a possibly-compromised account, and an attacker who was signed
// in when the reset happened keeps their session otherwise — the
// refresh token is independent of the password, so nothing about
// changing it disturbs them.
//
// The auth_log rows are left in place. They are the audit trail, and
// Browse already derives IsActive from the refresh token's revoked
// flag, so revoking is enough to make the sessions read as ended
// without destroying the record that they existed.
//
// Returns the number revoked. Errors on individual tokens are logged
// and skipped rather than aborting: a partial revocation is strictly
// better than stopping at the first failure and leaving the rest
// live.
//
// Spares nothing, including the caller's own session. That is right
// for recovery flows, which do not run inside a session worth keeping
// — password reset is unauthenticated by construction. Authenticated
// callers want DeleteAllSessionsExcept.
func (h *authLogHandler) DeleteAllSessions(user *entities.User) (int, error) {
	return h.DeleteAllSessionsExcept(user, 0)
}

// DeleteAllSessionsExcept revokes every active refresh token a person
// holds apart from one.
//
// Same purge as DeleteAllSessions, for the flows that run inside a
// session worth keeping. An authenticated password change is the case
// it exists for: the person is deliberately signing every other device
// out, and signing out the tab they are doing it from turns a security
// action into a logout.
//
// exceptRefreshTokenID is the caller's own refresh token — in practice
// viewer.OAuthRefreshTokenID, the claim that identifies the session
// this request arrived on. Pass 0 to spare nothing, which is what
// DeleteAllSessions does.
//
// A zero value cannot be an accidental match: rows with no refresh
// token are skipped before the comparison, so "spare token 0" and
// "spare nothing" mean the same thing rather than colliding.
func (h *authLogHandler) DeleteAllSessionsExcept(
	user *entities.User,
	exceptRefreshTokenID int64,
) (int, error) {
	authLogs, err := h.Repos.AuthLogs.Browse(user)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	revoked := 0

	for i := range authLogs {
		if authLogs[i].OAuthRefreshTokenID == 0 {
			continue
		}

		if authLogs[i].OAuthRefreshTokenID == exceptRefreshTokenID {
			continue
		}

		rt, err := h.Repos.OAuthRefreshTokens.ReadByID(authLogs[i].OAuthRefreshTokenID)
		if err != nil {
			continue
		}
		if rt.Revoked || !rt.ExpiresAt.After(now) {
			continue
		}

		if err := h.Repos.OAuthRefreshTokens.Revoke(rt.ID); err != nil {
			log.Println("auth log: revoke during session purge failed:", err)
			continue
		}
		revoked++
	}

	return revoked, nil
}

// DeleteRedundantSessions revokes refresh tokens beyond the active session cap.
func (h *authLogHandler) DeleteRedundantSessions(c echo.Context, user *entities.User) error {
	authLogs, err := h.Repos.AuthLogs.Browse(user)
	if err != nil {
		return err
	}

	now := time.Now()
	count := 0
	for i := range authLogs {
		if authLogs[i].OAuthRefreshTokenID == 0 {
			continue
		}

		rt, err := h.Repos.OAuthRefreshTokens.ReadByID(authLogs[i].OAuthRefreshTokenID)
		if err != nil {
			continue
		}
		if rt.Revoked || !rt.ExpiresAt.After(now) {
			continue
		}

		count++
		if count > constants.AuthLogMaxNumActiveSessions {
			if err := h.Repos.OAuthRefreshTokens.Revoke(rt.ID); err != nil {
				return err
			}
		}
	}

	return nil
}
