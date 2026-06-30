package email

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
	"github.com/nurik/Dev/repos/mengu-backend/internal/organization"
)

var ErrInvalidSecret = errors.New("invalid webhook secret")

type Service struct {
	repo    *Repository
	orgRepo *organization.Repository
}

func NewService(repo *Repository, orgRepo *organization.Repository) *Service {
	return &Service{repo: repo, orgRepo: orgRepo}
}

type WebhookPayload struct {
	From        string              `json:"from" example:"sender@company.com" format:"email"`
	Subject     string              `json:"subject" example:"Contract Review Meeting"`
	Body        string              `json:"body" example:"We need to review the updated contract terms..."`
	Attachments []AttachmentPayload `json:"attachments,omitempty"`
}

type AttachmentPayload struct {
	Filename      string `json:"filename" example:"contract.pdf"`
	ContentType   string `json:"content_type" example:"application/pdf"`
	Size          int64  `json:"size" example:"102400"`
	URL           string `json:"url" example:"https://storage.example.com/contract.pdf"`
	Content       string `json:"content,omitempty" example:"Extracted attachment text"`
	Data          string `json:"data,omitempty" example:"Raw attachment text"`
	ContentBase64 string `json:"content_base64,omitempty" example:"base64-encoded attachment data"`
}
type WebhookResult struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

func (s *Service) ProcessWebhook(ctx context.Context, secret string, payload *WebhookPayload) (*WebhookResult, error) {
	org, err := s.orgRepo.GetByWebhookSecret(ctx, secret)
	if err != nil {
		return nil, ErrInvalidSecret
	}

	metadata := map[string]interface{}{
		"sender":      payload.From,
		"subject":     payload.Subject,
		"attachments": payload.Attachments,
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	evt, err := s.repo.Create(ctx, CreateEventParams{
		OrgID:      org.ID,
		Source:     "email",
		RawContent: payload.Body,
		Metadata:   metaJSON,
	})
	if err != nil {
		return nil, err
	}

	return &WebhookResult{
		EventID: evt.ID,
		Status:  evt.Status,
	}, nil
}

func (s *Service) CreateEventFromEmail(ctx context.Context, orgID, source, sender, subject, body string, attachments []AttachmentPayload) (*WebhookResult, error) {
	metadata := map[string]interface{}{
		"sender":      sender,
		"subject":     subject,
		"attachments": attachments,
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	evt, err := s.repo.Create(ctx, CreateEventParams{
		OrgID:      orgID,
		Source:     source,
		RawContent: body,
		Metadata:   metaJSON,
	})
	if err != nil {
		return nil, err
	}

	return &WebhookResult{
		EventID: evt.ID,
		Status:  evt.Status,
	}, nil
}

func (s *Service) GetEvent(ctx context.Context, id, orgID string) (*model.IncomingEvent, error) {
	return s.repo.GetByID(ctx, id, orgID)
}

type ListInput struct {
	OrgID  string
	Status string
	Page   int
	Limit  int
}

func (s *Service) ListEvents(ctx context.Context, input ListInput) (*ListEventsResult, error) {
	return s.repo.List(ctx, ListEventsParams{
		OrgID:  input.OrgID,
		Status: input.Status,
		Page:   input.Page,
		Limit:  input.Limit,
	})
}

func (s *Service) Reanalyze(ctx context.Context, id, orgID string) error {
	_, err := s.repo.GetByID(ctx, id, orgID)
	if err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, id, orgID, "new")
}
