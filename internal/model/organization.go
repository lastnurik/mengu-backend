package model

import "time"

type Organization struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	WebhookSecret string    `json:"-"`
	Plan          string    `json:"plan"`
	CreatedAt     time.Time `json:"created_at"`
}
