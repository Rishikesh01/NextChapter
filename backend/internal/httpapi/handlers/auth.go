// Package handlers contains the gin route handlers. One file per
// resource, matching the openapi tags (auth, series, entries, meta).
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/httpapi/api"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// AuthDeps groups the dependencies the auth handlers need.
type AuthDeps struct {
	Users        *users.Service
	Auth         *auth.Service
	Logger       *zap.Logger
	HasEnvBoot   bool // env-var bootstrap enabled; closes /auth/register entirely.
	CookieDomain string
	CookieSecure bool
	Now          func() time.Time
}

// UserResponse is the JSON shape returned for /auth/me, /auth/login, /auth/register.
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func userToJSON(id int64, username string, createdAt time.Time) UserResponse {
	return UserResponse{ID: id, Username: username, CreatedAt: createdAt}
}

// Register implements POST /auth/register, the open-registration window.
// Returns 404 when the window is closed (users exist, or env-var
// bootstrap is configured).
func (d AuthDeps) Register(c *gin.Context) {
	if d.HasEnvBoot {
		c.AbortWithStatusJSON(http.StatusNotFound, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeNotFound,
			Message: "not found",
		}})
		return
	}
	count, err := d.Users.Count(c.Request.Context())
	if err != nil {
		d.Logger.Error("register: count users", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if count > 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeNotFound,
			Message: "not found",
		}})
		return
	}
	var req users.RegisterParams
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, api.ErrorBody{Error: api.ErrorDetail{
				Code:    api.CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	d.Logger.Warn("open registration window: creating first user",
		zap.String("username", req.Username),
	)
	u, err := d.Users.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, users.ErrUsernameTaken) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, api.ErrorBody{Error: api.ErrorDetail{
				Code:    api.CodeValidation,
				Message: "username already taken",
				Fields:  map[string]string{"username": "already taken"},
			}})
			return
		}
		d.Logger.Error("register: create user", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	tok, err := d.Auth.CreateSession(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("register: mint session", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	d.setSessionCookie(c, tok.Raw, int(auth.SessionDuration.Seconds()))
	c.JSON(http.StatusCreated, userToJSON(u.ID, u.Username, u.CreatedAt))
}

// Login implements POST /auth/login.
func (d AuthDeps) Login(c *gin.Context) {
	var req auth.LoginParams
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, api.ErrorBody{Error: api.ErrorDetail{
				Code:    api.CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	u, err := d.Users.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, api.ErrorBody{Error: api.ErrorDetail{
				Code:    api.CodeUnauthorized,
				Message: "invalid credentials",
			}})
			return
		}
		d.Logger.Error("login: get user", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if err := auth.VerifyPassword(u.PasswordHash, req.Password); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeUnauthorized,
			Message: "invalid credentials",
		}})
		return
	}
	tok, err := d.Auth.CreateSession(c.Request.Context(), u.ID)
	if err != nil {
		d.Logger.Error("login: mint session", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	d.setSessionCookie(c, tok.Raw, int(auth.SessionDuration.Seconds()))
	c.JSON(http.StatusOK, userToJSON(u.ID, u.Username, u.CreatedAt))
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
	u, ok := auth.UserFromContext(c.Request.Context())
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeUnauthorized,
			Message: "missing or invalid credentials",
		}})
		return
	}
	c.JSON(http.StatusOK, userToJSON(u.ID, u.Username, u.CreatedAt))
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

// APITokenCreatedResponse is the POST /auth/tokens response: the stored
// token fields plus the raw plaintext token, which the server returns
// exactly once.
type APITokenCreatedResponse struct {
	APITokenResponse
	Token string `json:"token"`
}

func tokenToJSON(t auth.Token) APITokenResponse {
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
	u, _ := auth.UserFromContext(c.Request.Context())
	var req auth.CreateTokenParams
	if err := c.ShouldBindJSON(&req); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, api.ErrorBody{Error: api.ErrorDetail{
				Code:    api.CodeValidation,
				Message: "invalid request",
				Fields:  validationFieldsFromErr(verr),
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeBadRequest,
			Message: "invalid request body",
		}})
		return
	}
	tok, err := d.Auth.CreateAPI(c.Request.Context(), u.ID, req)
	if err != nil {
		d.Logger.Error("create token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	body := APITokenCreatedResponse{APITokenResponse: tokenToJSON(tok.Token), Token: tok.Raw}
	c.JSON(http.StatusCreated, body)
}

// DeleteToken implements DELETE /auth/tokens/{id}.
func (d AuthDeps) DeleteToken(c *gin.Context) {
	u, _ := auth.UserFromContext(c.Request.Context())
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeNotFound,
			Message: "not found",
		}})
		return
	}
	ok, err := d.Auth.DeleteAPI(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		d.Logger.Error("delete token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, api.ErrorBody{Error: api.ErrorDetail{
			Code:    api.CodeNotFound,
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
