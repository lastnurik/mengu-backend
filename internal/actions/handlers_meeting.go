package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nurik/Dev/repos/mengu-backend/internal/calendar"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type MeetingHandler struct {
	calClient *calendar.Client
}

func NewMeetingHandler(calClient *calendar.Client) *MeetingHandler {
	return &MeetingHandler{calClient: calClient}
}

func (h *MeetingHandler) Handle(ctx context.Context, orgID, eventID string, action ai.Action) error {
	var data struct {
		Title         string `json:"title"`
		MeetingTitle  string `json:"meeting_title"`
		Summary       string `json:"summary"`
		Datetime      string `json:"datetime"`
		Duration      int    `json:"duration_minutes"`
	}
	if err := json.Unmarshal(action.Data, &data); err != nil {
		return fmt.Errorf("invalid meeting data: %w", err)
	}
	if data.Title == "" {
		data.Title = data.MeetingTitle
	}
	if data.Title == "" {
		data.Title = data.Summary
	}
	if data.Title == "" {
		data.Title = "Scheduled Meeting"
	}
	if data.Datetime == "" {
		return fmt.Errorf("meeting datetime is required")
	}

	startTime, err := time.Parse(time.RFC3339, data.Datetime)
	if err != nil {
		return fmt.Errorf("invalid datetime format: %w", err)
	}

	if data.Duration == 0 {
		data.Duration = 30
	}
	endTime := startTime.Add(time.Duration(data.Duration) * time.Minute)

	_, err = h.calClient.CreateEvent(ctx, orgID, data.Title, "Meeting from event: "+eventID,
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), nil)
	if err != nil {
		return fmt.Errorf("failed to create calendar event: %w", err)
	}

	return nil
}
