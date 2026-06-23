package gmail

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/email"
	"google.golang.org/api/idtoken"
)

var gmailPubSubAudience = ""

type Handler struct {
	watchRepo *Repository
	emailSvc  *email.Service
	apiClient *APIClient
	logger    *slog.Logger
}

func NewHandler(watchRepo *Repository, apiClient *APIClient, emailSvc *email.Service, logger *slog.Logger) *Handler {
	return &Handler{watchRepo: watchRepo, apiClient: apiClient, emailSvc: emailSvc, logger: logger}
}

type pubSubPushBody struct {
	Message struct {
		Data        string            `json:"data"`
		MessageID   string            `json:"messageId"`
		Attributes  map[string]string `json:"attributes"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

type pubSubData struct {
	EmailAddress string      `json:"emailAddress"`
	HistoryID    json.Number `json:"historyId"`
}

// Webhook godoc
// @Summary      Gmail push notification
// @Description  Receives Pub/Sub push notifications from Google for Gmail mailbox changes. Verifies Google-issued JWT, fetches new messages from Gmail API and creates incoming_events.
// @Tags         Gmail
// @Accept       json
// @Produce      json
// @Param        request  body  pubSubPushBody  true  "Pub/Sub push notification envelope"
// @Success      200  {object}  object{}
// @Router       /webhooks/gmail [post]
func (h *Handler) Webhook(c *gin.Context) {
	if err := h.verifyGmailJWT(c); err != nil {
		h.logger.Warn("gmail webhook: JWT verification failed", "error", err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	var body pubSubPushBody
	if err := c.ShouldBindJSON(&body); err != nil {
		h.logger.Warn("gmail webhook: invalid push body", "error", err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(body.Message.Data)
	if err != nil {
		h.logger.Warn("gmail webhook: failed to decode data", "error", err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	var data pubSubData
	if err := json.Unmarshal(decoded, &data); err != nil {
		h.logger.Warn("gmail webhook: failed to parse data", "error", err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	watch, err := h.watchRepo.GetByEmailAddress(c.Request.Context(), data.EmailAddress)
	if err != nil {
		h.logger.Warn("gmail webhook: no watch found for email", "email", data.EmailAddress)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	storedHist, _ := strconv.ParseUint(watch.HistoryID, 10, 64)
	notifHist, _ := strconv.ParseUint(data.HistoryID.String(), 10, 64)
	if notifHist <= storedHist {
		h.logger.Info("gmail webhook: skipping stale notification", "org_id", watch.OrgID, "email", data.EmailAddress, "stored", storedHist, "notified", notifHist)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	h.logger.Info("gmail webhook: processing notification", "org_id", watch.OrgID, "email", data.EmailAddress, "start_history", watch.HistoryID, "new_history", data.HistoryID)

	historyRecords, err := h.apiClient.GetHistory(c.Request.Context(), watch.OrgID, data.EmailAddress, watch.HistoryID)
	if err != nil {
		h.logger.Error("gmail webhook: failed to fetch history", "org_id", watch.OrgID, "error", err)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	processed := 0
	for _, record := range historyRecords {
		for _, msg := range record.Messages {
			fullMsg, err := h.apiClient.GetMessage(c.Request.Context(), watch.OrgID, data.EmailAddress, msg.Id)
			if err != nil {
				h.logger.Error("gmail webhook: failed to fetch message", "message_id", msg.Id, "error", err)
				continue
			}

			sender, subject, body, extractedAtts, err := ExtractEmailFromMessageWithAttachments(fullMsg)
			if err != nil {
				h.logger.Error("gmail webhook: failed to extract email fields", "message_id", msg.Id, "error", err)
				continue
			}
			if sender == "" || subject == "" {
				h.logger.Warn("gmail webhook: skipping message with missing fields", "message_id", msg.Id)
				continue
			}

			var payloadAtts []email.AttachmentPayload
			for _, a := range extractedAtts {
				payloadAtts = append(payloadAtts, email.AttachmentPayload{
					Filename:    a.Filename,
					ContentType: a.ContentType,
					Size:        a.Size,
					URL:         a.URL,
				})
			}

			_, err = h.emailSvc.CreateEventFromEmail(c.Request.Context(), watch.OrgID, "gmail", sender, subject, body, payloadAtts)
			if err != nil {
				h.logger.Error("gmail webhook: failed to create event", "message_id", msg.Id, "error", err)
				continue
			}

			processed++
		}
	}

	if err := h.watchRepo.UpdateHistoryID(c.Request.Context(), watch.OrgID, data.HistoryID.String()); err != nil {
		h.logger.Error("gmail webhook: failed to update history ID", "org_id", watch.OrgID, "error", err)
	}

	h.logger.Info("gmail webhook: processed messages", "org_id", watch.OrgID, "email", data.EmailAddress, "total", processed, "history_records", len(historyRecords))

	c.JSON(http.StatusOK, gin.H{})
}

type WatchRequest struct {
	EmailAddress string `json:"email_address" example:"user@company.com" format:"email"`
}

type WatchResponse struct {
	Status       string `json:"status" example:"watch_started"`
	EmailAddress string `json:"email_address" example:"user@company.com"`
	ExpiresAt    string `json:"expires_at" example:"2026-06-17T15:26:00Z"`
}

// InitiateWatch godoc
// @Summary      Start Gmail watch
// @Description  Initiates Gmail API watch for the authenticated user's organization — subscribes to mailbox push notifications (requires prior Google OAuth consent).
// @Tags         Gmail
// @Accept       json
// @Produce      json
// @Param        request  body  WatchRequest  true  "Email address to watch"
// @Success      200      {object}  WatchResponse
// @Failure      400      {object}  object{error=string,message=string}
// @Failure      500      {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /gmail/watch [post]
func (h *Handler) InitiateWatch(c *gin.Context) {
	orgID := c.GetString("org_id")

	emailAddress, err := h.apiClient.GetEmailAddress(c.Request.Context(), orgID)
	if err != nil {
		h.logger.Error("gmail get profile failed", "org_id", orgID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "gmail_not_connected", "message": "Connect Gmail first before enabling monitoring: " + err.Error()})
		return
	}

	historyID, err := h.apiClient.Watch(c.Request.Context(), orgID, emailAddress)
	if err != nil {
		h.logger.Error("gmail API watch failed", "org_id", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "watch_failed", "message": "Failed to initiate Gmail watch: " + err.Error()})
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	watch := &WatchRow{
		OrgID:        orgID,
		EmailAddress: emailAddress,
		HistoryID:    historyID,
		TopicName:    h.apiClient.TopicName(),
		ExpiresAt:    expiresAt,
	}

	if err := h.watchRepo.Upsert(c.Request.Context(), watch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to save watch"})
		return
	}

	h.logger.Info("gmail watch started", "org_id", orgID, "email", emailAddress)
	c.JSON(http.StatusOK, WatchResponse{
		Status:       "watch_started",
		EmailAddress: emailAddress,
		ExpiresAt:    expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) verifyGmailJWT(c *gin.Context) error {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return fmt.Errorf("missing Authorization header")
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return fmt.Errorf("invalid Authorization header format")
	}

	payload, err := idtoken.Validate(c.Request.Context(), parts[1], gmailPubSubAudience)
	if err != nil {
		return fmt.Errorf("JWT validation failed: %w", err)
	}

	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return fmt.Errorf("unexpected JWT issuer: %s", payload.Issuer)
	}

	if email := payload.Claims["email"]; email == "" {
		return fmt.Errorf("JWT missing email claim")
	}

	return nil
}
