package handlers

import (
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
		req    = requests.NewAuthlogRequest().Browse
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

	respAuthlogs := &responses.AuthlogResponse{}
	resp := respAuthlogs.MapAuthlogItems(authLogs)

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
	userAgentString := c.Request().Header.Get("User-Agent")
	clientOS := c.Request().Header.Get("X-Client-OS")
	clientApp := c.Request().Header.Get("X-Client-App")

	ua := useragent.Parse(userAgentString)

	authLog := entities.NewAuthLog()
	authLog.CreatedOn = time.Now()
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

	persisted, err := h.Repos.AuthLogs.Add(authLog)
	if err != nil {
		return nil, err
	}

	if err := h.Repos.AuthLogs.Trim(user); err != nil {
		return nil, err
	}

	return persisted, nil
}

// Delete force-logs-out a session by revoking its refresh token.
func (h *authLogHandler) Delete(c echo.Context) error {
	var (
		err    error
		viewer = getViewer(c)
		req    = requests.NewAuthlogRequest().Delete
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

	if authLog.OAuthRefreshTokenID != 0 {
		if err := h.Repos.OAuthRefreshTokens.Revoke(authLog.OAuthRefreshTokenID); err != nil {
			log.Println("authlog delete: revoke failed, continuing:", err)
		}
	}

	if err := h.Repos.AuthLogs.Delete(authLog.ID); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
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
func (h *authLogHandler) DeleteAllSessions(user *entities.User) (int, error) {
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

		rt, err := h.Repos.OAuthRefreshTokens.ReadByID(authLogs[i].OAuthRefreshTokenID)
		if err != nil {
			continue
		}
		if rt.Revoked || !rt.ExpiresAt.After(now) {
			continue
		}

		if err := h.Repos.OAuthRefreshTokens.Revoke(rt.ID); err != nil {
			log.Println("authlog: revoke during session purge failed:", err)
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
