package oauth

import "time"

type Token struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Provider     string    `json:"provider"`
	Scope        string    `json:"scope"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
