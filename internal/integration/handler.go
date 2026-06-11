package integration

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Handler struct {
	oauthRepo      *oauth.Repository
	googleCID      string
	googleCS       string
	oauthRedirect  string
	frontendURL    string
}

func NewHandler(oauthRepo *oauth.Repository, googleCID, googleCS, oauthRedirect, frontendURL string) *Handler {
	return &Handler{
		oauthRepo:     oauthRepo,
		googleCID:     googleCID,
		googleCS:      googleCS,
		oauthRedirect: oauthRedirect,
		frontendURL:   frontendURL,
	}
}

var providerScopes = map[string][]string{
	"gmail":    {"https://www.googleapis.com/auth/gmail.readonly"},
	"calendar": {"https://www.googleapis.com/auth/calendar.events"},
}

// List godoc
// @Summary      List integrations
// @Description  Returns connection status for all supported OAuth providers (gmail, calendar).
// @Tags         Integrations
// @Produce      json
// @Success      200  {array}   object{provider=string,connected=bool,scope=string}
// @Failure      500  {object}  object{error=string}
// @Security     Bearer
// @Router       /integrations [get]
func (h *Handler) List(c *gin.Context) {
	orgID := c.GetString("org_id")

	tokens, err := h.oauthRepo.ListByOrg(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	type integrationStatus struct {
		Provider   string `json:"provider"`
		Connected  bool   `json:"connected"`
		Scope      string `json:"scope,omitempty"`
	}

	connected := map[string]bool{}
	for _, t := range tokens {
		connected[t.Provider] = true
	}

	result := []integrationStatus{}
	for provider := range providerScopes {
		result = append(result, integrationStatus{
			Provider:  provider,
			Connected: connected[provider],
		})
	}

	c.JSON(http.StatusOK, result)
}

// OAuthURL godoc
// @Summary      Get integration OAuth URL
// @Description  Returns a Google OAuth URL for connecting a service (gmail or calendar). User must visit the URL to grant consent.
// @Tags         Integrations
// @Produce      json
// @Param        provider  query  string  true  "Provider name: gmail or calendar"
// @Success      200       {object}  object{url=string}
// @Failure      400       {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /integrations/oauth/url [get]
func (h *Handler) OAuthURL(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider_required", "message": "provider query param required (gmail, calendar)"})
		return
	}

	scopes, ok := providerScopes[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_provider", "message": "supported providers: gmail, calendar"})
		return
	}

	orgID := c.GetString("org_id")
	state := fmt.Sprintf("%s:%s:connect", provider, orgID)

	config := &oauth2.Config{
		ClientID:     h.googleCID,
		ClientSecret: h.googleCS,
		RedirectURL:  h.oauthRedirect,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}

	url := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	c.JSON(http.StatusOK, gin.H{"url": url})
}

// Disconnect godoc
// @Summary      Disconnect integration
// @Description  Disconnects a provider (gmail or calendar) by removing stored OAuth tokens.
// @Tags         Integrations
// @Produce      json
// @Param        provider  path  string  true  "Provider name: gmail or calendar"
// @Success      200       {object}  object{status=string}
// @Failure      400       {object}  object{error=string}
// @Security     Bearer
// @Router       /integrations/{provider} [delete]
func (h *Handler) Disconnect(c *gin.Context) {
	orgID := c.GetString("org_id")
	provider := c.Param("provider")

	if _, ok := providerScopes[provider]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_provider"})
		return
	}

	if err := h.oauthRepo.Delete(c.Request.Context(), orgID, provider); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "disconnected"})
}

// HandleCallback processes OAuth callback for service connection (gmail, calendar).
// Called from the auth OAuthCallback handler when state indicates a connect flow.
func (h *Handler) HandleCallback(ctx context.Context, orgID, provider, code string) error {
	scopes, ok := providerScopes[provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", provider)
	}

	config := &oauth2.Config{
		ClientID:     h.googleCID,
		ClientSecret: h.googleCS,
		RedirectURL:  h.oauthRedirect,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	scope := strings.Join(scopes, " ")
	tok := &oauth.Token{
		OrgID:        orgID,
		Provider:     provider,
		Scope:        scope,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
	}

	return h.oauthRepo.Upsert(ctx, tok)
}
