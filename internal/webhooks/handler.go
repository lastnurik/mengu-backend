package webhooks

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/email"
)

type Handler struct {
	svc *email.Service
}

func NewHandler(svc *email.Service) *Handler {
	return &Handler{svc: svc}
}

// Email godoc
// @Summary      Ingest email via webhook
// @Description  Receive incoming email from external email service. Looks up organization by X-Webhook-Secret and stores as incoming_event.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        X-Webhook-Secret  header    string                       true  "Organization webhook secret"
// @Param        request           body      email.WebhookPayload         true  "Email payload"
// @Success      201               {object}  object{event_id=string,status=string}
// @Failure      400               {object}  object{error=string,message=string}
// @Failure      401               {object}  object{error=string,message=string}
// @Security     WebhookSecret
// @Router       /webhooks/email [post]
func (h *Handler) Email(c *gin.Context) {
	secret := c.GetHeader("X-Webhook-Secret")
	if secret == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized", "message": "Invalid or missing X-Webhook-Secret",
		})
		return
	}

	var payload email.WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_payload", "message": "Missing required fields: from, subject, body",
		})
		return
	}

	if payload.From == "" || payload.Subject == "" || payload.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_payload", "message": "Missing required fields: from, subject, body",
		})
		return
	}

	result, err := h.svc.ProcessWebhook(c.Request.Context(), secret, &payload)
	if err != nil {
		if errors.Is(err, email.ErrInvalidSecret) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized", "message": "Invalid or missing X-Webhook-Secret",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "internal_error", "message": "Failed to process webhook",
		})
		return
	}

	c.JSON(http.StatusCreated, result)
}
