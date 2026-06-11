package gmail

import (
	"context"
	"log/slog"
	"time"
)

type RenewalService struct {
	repo     *Repository
	api      *APIClient
	logger   *slog.Logger
	interval time.Duration
}

func NewRenewalService(repo *Repository, api *APIClient, logger *slog.Logger, interval time.Duration) *RenewalService {
	return &RenewalService{repo: repo, api: api, logger: logger, interval: interval}
}

func (s *RenewalService) Run(ctx context.Context) {
	s.logger.Info("gmail watch renewal started", "check_interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("gmail watch renewal stopped")
			return
		case <-ticker.C:
			s.renewExpiring(ctx)
		}
	}
}

func (s *RenewalService) renewExpiring(ctx context.Context) {
	watches, err := s.repo.ListExpiring(ctx, 24*time.Hour)
	if err != nil {
		s.logger.Error("gmail renewal: failed to list expiring watches", "error", err)
		return
	}

	for _, watch := range watches {
		s.logger.Info("gmail renewal: renewing watch", "org_id", watch.OrgID, "email", watch.EmailAddress)

		historyID, err := s.api.Watch(ctx, watch.OrgID, watch.EmailAddress)
		if err != nil {
			s.logger.Error("gmail renewal: API watch failed", "org_id", watch.OrgID, "error", err)
			continue
		}

		newExpiry := time.Now().Add(7 * 24 * time.Hour)
		watch.ExpiresAt = newExpiry
		watch.HistoryID = historyID

		if err := s.repo.Upsert(ctx, &watch); err != nil {
			s.logger.Error("gmail renewal: failed to save renewed watch", "org_id", watch.OrgID, "error", err)
		}
	}
}
