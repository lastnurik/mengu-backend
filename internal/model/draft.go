package model

import "time"

type Draft struct {
	ID         string    `json:"id" example:"draft_001" format:"uuid"`
	OrgID      string    `json:"org_id" example:"org_123" format:"uuid"`
	EventID    string    `json:"event_id" example:"evt_001" format:"uuid"`
	Recipients string    `json:"recipients" example:"partner@company.com"`
	Subject    string    `json:"subject" example:"Meeting Confirmation"`
	Body       string    `json:"body" example:"Dear Partner,\n\nI confirm our meeting..."`
	Status     string    `json:"status" example:"pending_review" enums:"pending_review,approved,sent"`
	CreatedAt  time.Time `json:"created_at" example:"2026-06-10T12:03:00Z"`
	UpdatedAt  time.Time `json:"updated_at" example:"2026-06-10T12:07:00Z"`
}
