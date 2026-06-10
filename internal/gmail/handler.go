package gmail

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/email"
)

type Handler struct {
	watchRepo *Repository
	_         *email.Service
	logger    *slog.Logger
}

func NewHandler(watchRepo *Repository, _ *email.Service, logger *slog.Logger) *Handler {
	return &Handler{watchRepo: watchRepo, logger: logger}
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
	EmailAddress string `json:"emailAddress"`
	HistoryID    string `json:"historyId"`
}

// Webhook godoc
// @Summary      Gmail push notification
// @Description  Receives Pub/Sub push notifications from Google for Gmail mailbox changes (new email, label changes). Idempotent: responds 200 immediately after logging and updates historyId.
// @Tags         Gmail
// @Accept       json
// @Produce      json
// @Param        request  body  pubSubPushBody  true  "Pub/Sub push notification envelope"
// @Success      200  {object}  object{}
// @Router       /webhooks/gmail [post]
func (h *Handler) Webhook(c *gin.Context) {
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

	h.watchRepo.UpdateHistoryID(c.Request.Context(), watch.OrgID, data.HistoryID)

	h.logger.Info("gmail webhook: processing notification", "org_id", watch.OrgID, "email", data.EmailAddress)

	_ = watch
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
	var req WatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "email_address is required"})
		return
	}

	orgID := c.GetString("org_id")

	existing, err := h.watchRepo.GetByOrgID(c.Request.Context(), orgID)
	if err == nil && existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "watch_already_active",
			"message": "A Gmail watch is already active for this organization",
		})
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	watch := &WatchRow{
		OrgID:        orgID,
		EmailAddress: req.EmailAddress,
		HistoryID:    "1",
		TopicName:    "mengu-gmail-topic",
		ExpiresAt:    expiresAt,
	}

	if err := h.watchRepo.Upsert(c.Request.Context(), watch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to initiate watch"})
		return
	}

	c.JSON(http.StatusOK, WatchResponse{
		Status:       "watch_started",
		EmailAddress: req.EmailAddress,
		ExpiresAt:    expiresAt.Format(time.RFC3339),
	})
}
