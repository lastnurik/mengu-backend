package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrUserNotFound       = errors.New("user not found")
)

type Service struct {
	repo         *Repository
	jwtSecret    string
	accessTTL    time.Duration
	refreshTTL   time.Duration
	googleCID    string
	googleCS     string
	microsoftCID string
	microsoftCS  string
}

func NewService(repo *Repository, jwtSecret string, accessTTL, refreshTTL time.Duration,
	googleCID, googleCS, microsoftCID, microsoftCS string) *Service {
	return &Service{
		repo:         repo,
		jwtSecret:    jwtSecret,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		googleCID:    googleCID,
		googleCS:     googleCS,
		microsoftCID: microsoftCID,
		microsoftCS:  microsoftCS,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
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

func (s *Service) handleOAuth(_ context.Context, _ string, _ string) (*TokenPair, error) {
	return &TokenPair{
		AccessToken:  "placeholder",
		RefreshToken: "placeholder",
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
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
