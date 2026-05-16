// Package handlers contains the gin route handlers. One file per
// resource, matching the openapi tags (auth, series, entries, meta).
//
// Import discipline: handlers depend only on [internal/models] for
// cross-package types and on [constants]. Domain packages (auth,
// users, series, entries) are NOT imported here — services arrive
// pre-wired via the *Deps structs from [internal/httpapi/router.go],
// and error comparisons go through the canonical sentinels in
// [internal/models].
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/models"
)

// sessionCookieMaxAge is the Max-Age set on the session cookie. Kept
// here (not in [internal/auth]) so handlers do not import the auth
// package; the value mirrors auth.SessionDuration (30 days). If they
// drift the integration tests will pin the bug.
const sessionCookieMaxAge = 30 * 24 * 60 * 60 // seconds

// AuthDeps groups the dependencies the auth handlers need. The
// service fields are typed against [models] interfaces so handlers
// stay decoupled from the auth / users packages.
type AuthDeps struct {
	Users        models.UsersService
	Auth         models.AuthService
	Logger       *zap.Logger
	HasEnvBoot   bool // env-var bootstrap enabled; closes /auth/register entirely.
	CookieDomain string
	CookieSecure bool
	Now          func() time.Time
}

// UserResponse is the JSON shape returned for /auth/me, /auth/login,
// /auth/register.
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func userToJSON(u models.User) UserResponse {
	return UserResponse{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt}
}

// Register implements POST /auth/register, the open-registration
// window. Returns 404 when the window is closed (users exist, or
// env-var bootstrap is configured).
func (d AuthDeps) Register(c *gin.Context) {
	if d.HasEnvBoot {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	count, err := d.Users.Count(c.Request.Context())
	if err != nil {
		d.Logger.Error("register: count users", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if count > 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	var req models.Registration
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	d.Logger.Warn("open registration window: creating first user",
		zap.String("username", req.Username),
	)
	u, err := d.Users.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, models.ErrUsernameTaken) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "username already taken",
				Fields:  map[string]string{"username": "already taken"},
			}})
			return
		}
		d.Logger.Error("register: create user", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	tok, err := d.Auth.CreateSession(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("register: mint session", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	d.setSessionCookie(c, tok.Raw, sessionCookieMaxAge)
	c.JSON(http.StatusCreated, userToJSON(u))
}

// Login implements POST /auth/login.
func (d AuthDeps) Login(c *gin.Context) {
	var creds models.Credentials
	if err := c.ShouldBindJSON(&creds); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	u, err := d.Users.Authenticate(c.Request.Context(), creds)
	if err != nil {
		// Username-miss and password-miss both collapse to 401 so
		// callers cannot enumerate accounts. Anything else is logged
		// and 500'd.
		if errors.Is(err, models.ErrUserNotFound) || errors.Is(err, models.ErrInvalidCredentials) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorBody{Error: ErrorDetail{
				Code:    CodeUnauthorized,
				Message: "invalid credentials",
			}})
			return
		}
		d.Logger.Error("login: authenticate", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	tok, err := d.Auth.CreateSession(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("login: mint session", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	d.setSessionCookie(c, tok.Raw, sessionCookieMaxAge)
	c.JSON(http.StatusOK, userToJSON(u))
}

// Logout implements POST /auth/logout. Best-effort: invalidates the
// session row (if cookie was the auth method) and clears the cookie.
func (d AuthDeps) Logout(c *gin.Context) {
	if cookie, err := c.Cookie(constants.SessionCookieName); err == nil && cookie != "" {
		if err := d.Auth.DeleteSession(c.Request.Context(), cookie); err != nil {
			d.Logger.Warn("logout: delete session", zap.Error(err))
		}
	}
	d.clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

// Me implements GET /auth/me.
func (d AuthDeps) Me(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorBody{Error: ErrorDetail{
			Code:    CodeUnauthorized,
			Message: "missing or invalid credentials",
		}})
		return
	}
	c.JSON(http.StatusOK, userToJSON(u))
}

// APITokenResponse is the JSON shape for a stored API token. Used by
// the POST /auth/tokens response (embedded in [APITokenCreatedResponse]).
type APITokenResponse struct {
	ID         int64      `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// APITokenCreatedResponse is the POST /auth/tokens response: the
// stored token fields plus the raw plaintext token, which the server
// returns exactly once.
type APITokenCreatedResponse struct {
	APITokenResponse
	Token string `json:"token"`
}

func tokenToJSON(t models.Token) APITokenResponse {
	return APITokenResponse{
		ID:         t.ID,
		Label:      t.Label,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
	}
}

// CreateToken implements POST /auth/tokens. Returns the raw token
// exactly once.
func (d AuthDeps) CreateToken(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var req models.NewToken
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
				Code:    CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, ErrorBody{Error: ErrorDetail{
			Code:    CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	tok, err := d.Auth.CreateAPI(c.Request.Context(), u.ID, req)
	if err != nil {
		d.Logger.Error("create token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	body := APITokenCreatedResponse{APITokenResponse: tokenToJSON(tok.Token), Token: tok.Raw}
	c.JSON(http.StatusCreated, body)
}

// DeleteToken implements DELETE /auth/tokens/{id}.
func (d AuthDeps) DeleteToken(c *gin.Context) {
	u, _ := models.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	ok, err := d.Auth.DeleteAPI(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		d.Logger.Error("delete token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	c.Status(http.StatusNoContent)
}

func (d AuthDeps) setSessionCookie(c *gin.Context, raw string, maxAgeSec int) {
	// SameSite=Lax matches ADR-0001.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(constants.SessionCookieName, raw, maxAgeSec, "/", d.CookieDomain, d.CookieSecure, true)
}

func (d AuthDeps) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(constants.SessionCookieName, "", -1, "/", d.CookieDomain, d.CookieSecure, true)
}
