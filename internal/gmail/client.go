package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/nurik/Dev/repos/mengu-backend/internal/oauth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type APIClient struct {
	oauthRepo     *oauth.Repository
	topicName     string
	oauthConfig   *oauth2.Config
}

func NewAPIClient(oauthRepo *oauth.Repository, topicName, clientID, clientSecret, redirectURL string) *APIClient {
	return &APIClient{
		oauthRepo: oauthRepo,
		topicName: topicName,
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				"https://www.googleapis.com/auth/gmail.readonly",
				"https://www.googleapis.com/auth/gmail.compose",
				"https://www.googleapis.com/auth/gmail.send",
			},
		},
	}
}

func (c *APIClient) tokenSource(ctx context.Context, orgID string) (oauth2.TokenSource, error) {
	tok, err := c.oauthRepo.GetByOrgAndProvider(ctx, orgID, "gmail")
	if err != nil {
		return nil, fmt.Errorf("no gmail token for org %s: %w", orgID, err)
	}

	return c.oauthConfig.TokenSource(ctx, &oauth2.Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.ExpiresAt,
	}), nil
}

func (c *APIClient) NewService(ctx context.Context, orgID string) (*gmail.Service, error) {
	ts, err := c.tokenSource(ctx, orgID)
	if err != nil {
		return nil, err
	}

	svc, err := gmail.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}

	return svc, nil
}

func (c *APIClient) TopicName() string {
	return c.topicName
}

func (c *APIClient) GetEmailAddress(ctx context.Context, orgID string) (string, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return "", err
	}
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return "", fmt.Errorf("failed to get gmail profile: %w", err)
	}
	return profile.EmailAddress, nil
}

func (c *APIClient) Watch(ctx context.Context, orgID, emailAddress string) (string, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return "", err
	}

	watchReq := &gmail.WatchRequest{
		TopicName:       c.topicName,
		LabelIds:        []string{"INBOX"},
		LabelFilterBehavior: "include",
	}

	resp, err := svc.Users.Watch(emailAddress, watchReq).Do()
	if err != nil {
		return "", fmt.Errorf("failed to watch gmail: %w", err)
	}

	return fmt.Sprintf("%d", resp.HistoryId), nil
}

func (c *APIClient) GetHistory(ctx context.Context, orgID, emailAddress, startHistoryID string) ([]*gmail.History, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return nil, err
	}

	var historyID uint64
	fmt.Sscanf(startHistoryID, "%d", &historyID)

	var allHistory []*gmail.History
	pageToken := ""
	for {
		call := svc.Users.History.List(emailAddress).StartHistoryId(historyID).HistoryTypes("messageAdded")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list history: %w", err)
		}
		allHistory = append(allHistory, resp.History...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return allHistory, nil
}

func (c *APIClient) GetMessage(ctx context.Context, orgID, emailAddress, messageID string) (*gmail.Message, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return nil, err
	}

	msg, err := svc.Users.Messages.Get(emailAddress, messageID).Format("full").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get message %s: %w", messageID, err)
	}
	return msg, nil
}

func (c *APIClient) CreateDraft(ctx context.Context, orgID, emailAddress, to, subject, bodyText string) (string, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return "", err
	}

	msg := &gmail.Message{
		Raw: encodeRFC2822(to, subject, bodyText),
	}

	draft, err := svc.Users.Drafts.Create(emailAddress, &gmail.Draft{Message: msg}).Do()
	if err != nil {
		return "", fmt.Errorf("failed to create gmail draft: %w", err)
	}
	return draft.Id, nil
}

func (c *APIClient) SendMessage(ctx context.Context, orgID, emailAddress, to, subject, bodyText string) (string, error) {
	svc, err := c.NewService(ctx, orgID)
	if err != nil {
		return "", err
	}

	msg := &gmail.Message{
		Raw: encodeRFC2822(to, subject, bodyText),
	}

	sent, err := svc.Users.Messages.Send(emailAddress, msg).Do()
	if err != nil {
		return "", fmt.Errorf("failed to send gmail message: %w", err)
	}
	return sent.Id, nil
}

func encodeRFC2822(to, subject, body string) string {
	raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", to, subject, body)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

type ExtractedAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	URL         string `json:"url"`
}

func ExtractEmailFromMessage(msg *gmail.Message) (sender, subject, body string, err error) {
	for _, header := range msg.Payload.Headers {
		switch strings.ToLower(header.Name) {
		case "from":
			sender = header.Value
		case "subject":
			subject = header.Value
		}
	}

	body = extractBody(msg.Payload)
	return
}

func ExtractEmailFromMessageWithAttachments(msg *gmail.Message) (sender, subject, date, body string, attachments []ExtractedAttachment, err error) {
	for _, header := range msg.Payload.Headers {
		switch strings.ToLower(header.Name) {
		case "from":
			sender = header.Value
		case "subject":
			subject = header.Value
		case "date":
			date = header.Value
		}
	}

	body = extractBody(msg.Payload)
	attachments = extractAttachments(msg.Payload)
	return
}

func extractAttachments(part *gmail.MessagePart) []ExtractedAttachment {
	var atts []ExtractedAttachment

	if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
		atts = append(atts, ExtractedAttachment{
			Filename:    part.Filename,
			ContentType: part.MimeType,
			Size:        int64(part.Body.Size),
			URL:         "",
		})
	}

	for _, sub := range part.Parts {
		atts = append(atts, extractAttachments(sub)...)
	}

	return atts
}

func extractBody(part *gmail.MessagePart) string {
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err != nil {
			data, _ = base64.StdEncoding.DecodeString(part.Body.Data)
		}
		return string(data)
	}

	if len(part.Parts) > 0 {
		for _, subPart := range part.Parts {
			if body := extractBody(subPart); body != "" {
				return body
			}
		}
	}

	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err != nil {
			data, _ = base64.StdEncoding.DecodeString(part.Body.Data)
		}
		return string(data)
	}

	return ""
}
