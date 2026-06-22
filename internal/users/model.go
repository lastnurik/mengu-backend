package users

import "time"

type UserListItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	AuthProvider string    `json:"auth_provider"`
	CreatedAt    time.Time `json:"created_at"`
}
