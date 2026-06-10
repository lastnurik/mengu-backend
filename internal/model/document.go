package model

import "time"

type Document struct {
	ID        string    `json:"id" example:"doc_001" format:"uuid"`
	OrgID     string    `json:"org_id" example:"org_123" format:"uuid"`
	EventID   string    `json:"event_id" example:"evt_001" format:"uuid"`
	Title     string    `json:"title" example:"Contract Draft v2"`
	Type      string    `json:"type" example:"contract" enums:"contract,proposal,report,memo"`
	Status    string    `json:"status" example:"draft" enums:"draft,final"`
	Content   string    `json:"content" example:"This agreement is entered into..."`
	CreatedAt time.Time `json:"created_at" example:"2026-06-10T12:03:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2026-06-10T12:06:00Z"`
}
