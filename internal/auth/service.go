package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
	"github.com/nurik/Dev/repos/mengu-backend/internal/organization"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrUserNotFound       = errors.New("user not found")
)

type Service struct {
	repo          *Repository
	orgRepo       *organization.Repository
	pool          *pgxpool.Pool
	jwtSecret     string
	accessTTL     time.Duration
	refreshTTL    time.Duration
	googleCID     string
	googleCS      string
	microsoftCID  string
	microsoftCS   string
	oauthRedirect string
	frontendURL   string
}

func NewService(repo *Repository, orgRepo *organization.Repository, pool *pgxpool.Pool, jwtSecret string, accessTTL, refreshTTL time.Duration,
	googleCID, googleCS, microsoftCID, microsoftCS, oauthRedirect, frontendURL string) *Service {
	return &Service{
		repo:          repo,
		orgRepo:       orgRepo,
		pool:          pool,
		jwtSecret:     jwtSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		googleCID:     googleCID,
		googleCS:      googleCS,
		microsoftCID:  microsoftCID,
		microsoftCS:   microsoftCS,
		oauthRedirect: oauthRedirect,
		frontendURL:   frontendURL,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type RegisterInput struct {
	OrgName  string `json:"org_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (*TokenPair, error) {
	if input.OrgName == "" || input.Email == "" || input.Password == "" {
		return nil, errors.New("org_name, email, and password are required")
	}

	slug := strings.ToLower(strings.ReplaceAll(input.OrgName, " ", "-")) + "-" + uuid.New().String()[:8]

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}
	webhookSecret := "whsec_" + hex.EncodeToString(secretBytes)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	org := &model.Organization{
		Name:          input.OrgName,
		Slug:          slug,
		WebhookSecret: webhookSecret,
		Plan:          "free",
	}
	row := tx.QueryRow(ctx,
		`INSERT INTO organization (name, slug, webhook_secret, plan) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		org.Name, org.Slug, org.WebhookSecret, org.Plan)
	if err := row.Scan(&org.ID, &org.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	user := &model.User{
		OrgID:        org.ID,
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: string(passwordHash),
		Role:         "admin",
		AuthProvider: "email",
	}
	row = tx.QueryRow(ctx,
		`INSERT INTO "user" (org_id, name, email, password_hash, role, auth_provider) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		user.OrgID, user.Name, user.Email, user.PasswordHash, user.Role, user.AuthProvider)
	if err := row.Scan(&user.ID, &user.CreatedAt); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.generateTokens(ctx, user)
}

func (s *Service) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokens(ctx, user)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	hash := hashToken(refreshToken)

	stored, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if stored.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}

	if time.Now().After(stored.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	if err := s.repo.RevokeRefreshToken(ctx, stored.ID); err != nil {
		return nil, fmt.Errorf("failed to revoke old token: %w", err)
	}

	user, err := s.repo.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return s.generateTokens(ctx, user)
}

func (s *Service) googleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.googleCID,
		ClientSecret: s.googleCS,
		RedirectURL:  s.oauthRedirect,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}
}

func (s *Service) OAuthURL(state string) string {
	config := s.googleOAuthConfig()
	return config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *Service) OAuthGoogle(ctx context.Context, code string) (*TokenPair, error) {
	if s.googleCID == "" || s.googleCS == "" {
		return nil, errors.New("google OAuth not configured")
	}
	return s.handleOAuth(ctx, code, "google")
}

func (s *Service) OAuthMicrosoft(ctx context.Context, code string) (*TokenPair, error) {
	if s.microsoftCID == "" || s.microsoftCS == "" {
		return nil, errors.New("microsoft OAuth not configured")
	}
	return s.handleOAuth(ctx, code, "microsoft")
}

type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (s *Service) handleOAuth(ctx context.Context, code, provider string) (*TokenPair, error) {
	config := s.googleOAuthConfig()

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	client := config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %w", err)
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	user, err := s.repo.GetByEmail(ctx, info.Email)
	if err != nil {
		return nil, fmt.Errorf("no account found for %s, please register first", info.Email)
	}

	return s.generateTokens(ctx, user)
}

func (s *Service) OAuthMicrosoftLogin(ctx context.Context, code string) (*TokenPair, error) {
	if s.microsoftCID == "" || s.microsoftCS == "" {
		return nil, errors.New("microsoft OAuth not configured")
	}
	return nil, errors.New("microsoft OAuth not yet implemented")
}

func (s *Service) generateTokens(ctx context.Context, user *model.User) (*TokenPair, error) {
	now := time.Now()

	accessClaims := jwt.MapClaims{
		"sub":    user.ID,
		"org_id": user.OrgID,
		"role":   user.Role,
		"iat":    now.Unix(),
		"exp":    now.Add(s.accessTTL).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessSigned, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshRaw := uuid.New().String()
	refreshHash := hashToken(refreshRaw)

	refreshToken := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: now.Add(s.refreshTTL),
	}
	if err := s.repo.CreateRefreshToken(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshRaw,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
