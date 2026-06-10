package organization

import (
	"context"
	"errors"

	"github.com/nurik/Dev/repos/mengu-backend/internal/model"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(ctx context.Context, id string) (*model.Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByWebhookSecret(ctx context.Context, secret string) (*model.Organization, error) {
	return s.repo.GetByWebhookSecret(ctx, secret)
}

func (s *Service) Update(ctx context.Context, org *model.Organization) error {
	if org.Name == "" {
		return errors.New("name cannot be empty")
	}
	if org.Plan == "" {
		return errors.New("plan cannot be empty")
	}
	return s.repo.Update(ctx, org)
}
