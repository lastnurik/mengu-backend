package model

import (
	"encoding/json"
	"time"
)

type IncomingEvent struct {
	ID         string          `json:"id" example:"evt_001" format:"uuid"`
	OrgID      string          `json:"org_id" example:"org_123" format:"uuid"`
	Source     string          `json:"source" example:"email" enums:"email,api,webhook,gmail"`
	RawContent string          `json:"raw_content" example:"We need to schedule a meeting..."`
	Metadata   json.RawMessage `json:"metadata" swaggertype:"object" example:"{\"sender\":\"partner@company.com\",\"subject\":\"Contract Review Meeting\"}"`
	Status     string          `json:"status" example:"completed" enums:"new,processing,completed,failed"`
	CreatedAt  time.Time       `json:"created_at" example:"2026-06-10T12:00:00Z"`
}

type AIAnalysis struct {
	ID          string          `json:"id" example:"analysis_001" format:"uuid"`
	OrgID       string          `json:"org_id" example:"org_123" format:"uuid"`
	EventID     string          `json:"event_id" example:"evt_001" format:"uuid"`
	Version     int             `json:"version" example:"1"`
	Intent      string          `json:"intent" example:"meeting_and_document_review"`
	Confidence  float32         `json:"confidence" example:"0.94"`
	Actions     json.RawMessage `json:"actions" swaggertype:"array,object" example:"[{\"type\":\"schedule_meeting\",\"data\":{\"title\":\"Contract Review Meeting\"}}]"`
	RawResponse json.RawMessage `json:"raw_response" swaggertype:"object"`
	CreatedAt   time.Time       `json:"created_at" example:"2026-06-10T12:01:00Z"`
}

type ActionLog struct {
	ID           string          `json:"id" example:"log_001" format:"uuid"`
	OrgID        string          `json:"org_id" example:"org_123" format:"uuid"`
	EventID      string          `json:"event_id" example:"evt_001" format:"uuid"`
	ActionType   string          `json:"action_type" example:"schedule_meeting" enums:"schedule_meeting,create_task,analyze_document,send_email_draft"`
	Payload      json.RawMessage `json:"payload" swaggertype:"object"`
	Status       string          `json:"status" example:"success" enums:"success,failed,skipped"`
	ErrorMessage *string         `json:"error_message,omitempty" example:"null"`
	CreatedAt    time.Time       `json:"created_at" example:"2026-06-10T12:02:00Z"`
}
