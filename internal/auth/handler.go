package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc           *Service
	integCallback func(ctx context.Context, orgID, provider, code string) error
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) SetIntegrationCallback(fn func(ctx context.Context, orgID, provider, code string) error) {
	h.integCallback = fn
}

// Register godoc
// @Summary      Register organization
// @Description  Create a new organization with an admin user. Returns JWT access and refresh tokens.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterInput  true  "Registration details"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Router       /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Invalid request body"})
		return
	}

	tokens, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "registration_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// Login godoc
// @Summary      Authenticate user
// @Description  Authenticate user via email and password. Returns JWT access and refresh tokens.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      object{email=string,password=string}  true  "Login credentials"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Failure      401      {object}  object{error=string,message=string}
// @Router       /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Email and password are required"})
		return
	}

	tokens, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials", "message": "Invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Login failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// Refresh godoc
// @Summary      Refresh access token
// @Description  Exchange a valid refresh token for a new access/refresh token pair.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      object{refresh_token=string}  true  "Refresh token"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Failure      401      {object}  object{error=string,message=string}
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Refresh token is required"})
		return
	}

	tokens, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrTokenExpired) || errors.Is(err, ErrTokenRevoked) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "message": "Invalid or expired refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Token refresh failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// OAuthGoogle godoc
// @Summary      Google OAuth login
// @Description  Authenticate or register via Google OAuth2 authorization code.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      object{code=string}  true  "Google OAuth authorization code"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Router       /auth/oauth/google [post]
func (h *Handler) OAuthGoogle(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Authorization code is required"})
		return
	}

	tokens, err := h.svc.OAuthGoogle(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth_failed", "message": "Google OAuth failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// MobileGoogleAuth godoc
// @Summary      Mobile Google Sign-In
// @Description  Authenticate via a Google ID token from the mobile Google Sign-In SDK. Returns Mengu JWT tokens as JSON — no redirect needed.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      object{id_token=string}  true  "Google ID token from native Sign-In SDK"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Router       /auth/mobile/google [post]
func (h *Handler) MobileGoogleAuth(c *gin.Context) {
	var req struct {
		IDToken string `json:"id_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "id_token is required"})
		return
	}

	tokens, err := h.svc.OAuthGoogleIDToken(c.Request.Context(), req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "auth_failed", "message": "Google ID token validation failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// OAuthMicrosoft godoc
// @Summary      Microsoft OAuth login
// @Description  Authenticate or register via Microsoft OAuth2 authorization code.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Param        request  body      object{code=string}  true  "Microsoft OAuth authorization code"
// @Success      200      {object}  object{access_token=string,refresh_token=string,token_type=string,expires_in=integer}
// @Failure      400      {object}  object{error=string,message=string}
// @Router       /auth/oauth/microsoft [post]
func (h *Handler) OAuthMicrosoft(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Authorization code is required"})
		return
	}

	tokens, err := h.svc.OAuthMicrosoft(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "oauth_failed", "message": "Microsoft OAuth failed"})
		return
	}

	c.JSON(http.StatusOK, tokens)
}

// OAuthURL godoc
// @Summary      Get OAuth URL
// @Description  Returns the Google OAuth authorization URL for login.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Success      200  {object}  object{url=string}
// @Router       /auth/oauth/url [get]
func (h *Handler) OAuthURL(c *gin.Context) {
	url := h.svc.OAuthURL("login")
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// OAuthCallback godoc
// @Summary      OAuth callback
// @Description  Handles OAuth callback from providers. State param determines purpose (login, gmail, calendar).
// @Tags         Authentication
// @Produce      json
// @Param        code   query  string  true  "Authorization code"
// @Param        state  query  string  true  "OAuth state (provider:purpose:org_id)"
// @Router       /auth/oauth/callback [get]
func (h *Handler) OAuthCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		c.Redirect(http.StatusFound, h.svc.frontendURL+"/login?error=missing_params")
		return
	}

	parts := strings.SplitN(state, ":", 4)

	if len(parts) >= 3 && parts[2] == "connect" {
		provider := parts[0]
		orgID := parts[1]

		// parts[3] (optional) is a URL-encoded mobile deep-link redirect.
		appRedirect := ""
		if len(parts) == 4 {
			appRedirect, _ = url.QueryUnescape(parts[3])
		}

		if h.integCallback != nil {
			if err := h.integCallback(c.Request.Context(), orgID, provider, code); err != nil {
				target := h.svc.frontendURL + "/settings?error=integration_failed&provider=" + provider
				if appRedirect != "" {
					target = appRedirect + "?error=integration_failed&provider=" + provider
				}
				c.Redirect(http.StatusFound, target)
				return
			}
		}

		target := h.svc.frontendURL + "/settings?integration=" + provider + "&status=connected"
		if appRedirect != "" {
			target = appRedirect + "?integration=" + provider + "&status=connected"
		}
		c.Redirect(http.StatusFound, target)
		return
	}

	tokens, err := h.svc.OAuthGoogle(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusFound, h.svc.frontendURL+"/login?error=oauth_failed")
		return
	}

	c.Redirect(http.StatusFound,
		h.svc.frontendURL+"/login?access_token="+tokens.AccessToken+"&refresh_token="+tokens.RefreshToken)
}
