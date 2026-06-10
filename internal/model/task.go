package model

import "time"

type Task struct {
	ID          string     `json:"id" example:"task_001" format:"uuid"`
	OrgID       string     `json:"org_id" example:"org_123" format:"uuid"`
	EventID     string     `json:"event_id" example:"evt_001" format:"uuid"`
	Title       string     `json:"title" example:"Prepare contract review"`
	Description string     `json:"description" example:"Review the updated contract terms from partner"`
	Status      string     `json:"status" example:"pending" enums:"pending,in_progress,completed,cancelled"`
	AssignedTo  string     `json:"assigned_to" example:"user_001" format:"uuid"`
	DueDate     *time.Time `json:"due_date,omitempty" example:"2026-06-17T00:00:00Z"`
	CreatedAt   time.Time  `json:"created_at" example:"2026-06-10T12:02:00Z"`
	UpdatedAt   time.Time  `json:"updated_at" example:"2026-06-10T12:05:00Z"`
}
