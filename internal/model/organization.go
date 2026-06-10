package model

import "time"

type Organization struct {
	ID            string    `json:"id" example:"org_123" format:"uuid"`
	Name          string    `json:"name" example:"Astana IT University" minLength:"1"`
	Slug          string    `json:"slug" example:"astana-it-university"`
	WebhookSecret string    `json:"-" swaggerignore:"true"`
	Plan          string    `json:"plan" example:"pro" enums:"free,pro,enterprise"`
	CreatedAt     time.Time `json:"created_at" example:"2026-01-01T00:00:00Z"`
}
