package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
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
