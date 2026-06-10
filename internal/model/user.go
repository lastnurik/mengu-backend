package model

import "time"

type User struct {
	ID           string    `json:"id" example:"user_001" format:"uuid"`
	OrgID        string    `json:"org_id" example:"org_123" format:"uuid"`
	Name         string    `json:"name" example:"John Doe"`
	Email        string    `json:"email" example:"admin@org.com" format:"email"`
	PasswordHash string    `json:"-" swaggerignore:"true"`
	Role         string    `json:"role" example:"admin" enums:"admin,manager,employee,viewer"`
	AuthProvider string    `json:"auth_provider" example:"email" enums:"email,google,microsoft"`
	CreatedAt    time.Time `json:"created_at" example:"2026-01-01T00:00:00Z"`
}

type RefreshToken struct {
	ID        string     `json:"id" example:"rt_001" format:"uuid"`
	UserID    string     `json:"user_id" example:"user_001" format:"uuid"`
	TokenHash string     `json:"-" swaggerignore:"true"`
	ExpiresAt time.Time  `json:"expires_at" example:"2026-01-08T00:00:00Z"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" example:"2026-01-02T00:00:00Z"`
	CreatedAt time.Time  `json:"created_at" example:"2026-01-01T00:00:00Z"`
}
