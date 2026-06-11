package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/nurik/Dev/repos/mengu-backend/internal/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Client struct {
	oauthRepo     *oauth.Repository
	oauthConfig   *oauth2.Config
}

func NewClient(oauthRepo *oauth.Repository, clientID, clientSecret, redirectURL string) *Client {
	return &Client{
		oauthRepo: oauthRepo,
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"https://www.googleapis.com/auth/calendar.events"},
		},
	}
}

func (c *Client) tokenSource(ctx context.Context, orgID string) (oauth2.TokenSource, error) {
	tok, err := c.oauthRepo.GetByOrgAndProvider(ctx, orgID, "calendar")
	if err != nil {
		return nil, fmt.Errorf("no calendar token for org %s: %w", orgID, err)
	}

	return c.oauthConfig.TokenSource(ctx, &oauth2.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.ExpiresAt,
	}), nil
}

func (c *Client) NewService(ctx context.Context, orgID string) (*calendar.Service, error) {
	ts, err := c.tokenSource(ctx, orgID)
	if err != nil {
		return nil, err
	}

	svc, err := calendar.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar service: %w", err)
	}

	return svc, nil
}

func (c *Client) ListEvents(ctx context.Context, orgID string, from, to time.Time) ([]*calendar.Event, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return nil, err
	}

	events, err := svc.Events.List("primary").
		TimeMin(from.Format(time.RFC3339)).
		TimeMax(to.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list calendar events: %w", err)
	}

	return events.Items, nil
}

func (c *Client) CreateEvent(ctx context.Context, orgID, summary, description, startTime, endTime string, attendees []string) (*calendar.Event, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return nil, err
	}

	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Start: &calendar.EventDateTime{
			DateTime: startTime,
			TimeZone: "UTC",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime,
			TimeZone: "UTC",
		},
	}

	for _, email := range attendees {
		event.Attendees = append(event.Attendees, &calendar.EventAttendee{Email: email})
	}

	created, err := svc.Events.Insert("primary", event).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar event: %w", err)
	}

	return created, nil
}
