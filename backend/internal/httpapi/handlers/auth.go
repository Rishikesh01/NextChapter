// Package handlers contains the gin route handlers. One file per
// resource, matching the openapi tags (auth, series, entries, meta).
//
// Import discipline: handlers depend on [internal/models] for the
// cross-package value types (User, Series, Entry, request/response
// shapes), on [constants], and on the four domain packages strictly
// for their service *interfaces* (auth.AuthService,
// users.UsersService, series.SeriesService, entries.EntriesService).
// The concrete services arrive pre-wired via the *Deps structs from
// [internal/httpapi/router.go]; error comparisons go through the
// canonical sentinels in [internal/models].
//
// Handlers do NOT define parallel *Response wrapper types or
// xToJSON mappers: domain models in [internal/models] carry json
// tags and are returned directly via c.JSON. The only types that
// live here are the error envelope ([ErrorBody] / [ErrorDetail]) and
// the binding-only request DTOs ([resourceIDUri]) that have no domain
// equivalent.
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
	"github.com/enable-it/nextchapter/backend/internal/models"
	"github.com/enable-it/nextchapter/backend/internal/users"
)

// sessionCookieMaxAge is the Max-Age set on the session cookie. Kept
// here (not in [internal/auth]) so handlers do not import the auth
// package; the value mirrors auth.SessionDuration (30 days). If they
// drift the integration tests will pin the bug.
const sessionCookieMaxAge = 30 * 24 * 60 * 60 // seconds

// AuthDeps groups the dependencies the auth handlers need. The
// service fields are typed against each domain package's interface so
// handlers stay decoupled from the concrete service implementations.
type AuthDeps struct {
	Users        users.UsersService
	Auth         auth.AuthService
	Logger       *zap.Logger
	CookieDomain string
	CookieSecure bool
}

// Register implements POST /auth/register. The route is unconditionally
// open: anyone can register an account. Env-var bootstrap pre-seeds the
// operator's account but does not gate this endpoint.
func (d AuthDeps) Register(c *gin.Context) {
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
	d.Logger.Info("user registered via /auth/register", zap.String("username", req.Username))
	u, err := d.Users.Register(c.Request.Context(), req)
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
	c.JSON(http.StatusCreated, u)
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
	u, err := d.Auth.Authenticate(c.Request.Context(), creds)
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
	c.JSON(http.StatusOK, u)
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
	c.JSON(http.StatusOK, u)
}

// CreateToken implements POST /auth/tokens. Returns the raw token
// exactly once via the Raw field on [models.APIToken] (json:"token").
func (d AuthDeps) CreateToken(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Auth.CreateToken"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
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
	tok, err := d.Auth.CreateAPIToken(c.Request.Context(), u.ID, req)
	if err != nil {
		d.Logger.Error("create token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	c.JSON(http.StatusCreated, tok)
}

// DeleteToken implements DELETE /auth/tokens/{id}.
func (d AuthDeps) DeleteToken(c *gin.Context) {
	u, ok := models.UserFromContext(c.Request.Context())
	if !ok {
		d.Logger.Error("handler: user missing from context", zap.String("handler", "Auth.DeleteToken"))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	var uri resourceIDUri
	if err := c.ShouldBindUri(&uri); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, ErrorBody{Error: ErrorDetail{
			Code:    CodeNotFound,
			Message: "not found",
		}})
		return
	}
	matched, err := d.Auth.DeleteAPIToken(c.Request.Context(), u.ID, uri.ID)
	if err != nil {
		d.Logger.Error("delete token", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{Error: ErrorDetail{
			Code:    CodeInternal,
			Message: "internal server error",
		}})
		return
	}
	if !matched {
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
