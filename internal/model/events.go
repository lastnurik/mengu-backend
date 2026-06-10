package model

import (
	"encoding/json"
	"time"
)

type IncomingEvent struct {
	ID         string          `json:"id"`
	OrgID      string          `json:"org_id"`
	Source     string          `json:"source"`
	RawContent string          `json:"raw_content"`
	Metadata   json.RawMessage `json:"metadata"`
	Status     string          `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AIAnalysis struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	EventID     string          `json:"event_id"`
	Version     int             `json:"version"`
	Intent      string          `json:"intent"`
	Confidence  float32         `json:"confidence"`
	Actions     json.RawMessage `json:"actions"`
	RawResponse json.RawMessage `json:"raw_response"`
	CreatedAt   time.Time       `json:"created_at"`
}

type ActionLog struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	EventID      string          `json:"event_id"`
	ActionType   string          `json:"action_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}
