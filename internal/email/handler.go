package email

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type EventsHandler struct {
	svc *Service
}

func NewEventsHandler(svc *Service) *EventsHandler {
	return &EventsHandler{svc: svc}
}

type eventListItem struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Subject   string `json:"subject"`
	Sender    string `json:"sender"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (h *EventsHandler) List(c *gin.Context) {
	orgID := c.GetString("org_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	status := c.Query("status")

	result, err := h.svc.ListEvents(c.Request.Context(), ListInput{
		OrgID:  orgID,
		Status: status,
		Page:   page,
		Limit:  perPage,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list events"})
		return
	}

	items := make([]eventListItem, 0, len(result.Events))
	for _, evt := range result.Events {
		var meta struct {
			Subject string `json:"subject"`
			Sender  string `json:"sender"`
		}
		if len(evt.Metadata) > 0 {
			json.Unmarshal(evt.Metadata, &meta)
		}
		items = append(items, eventListItem{
			ID:        evt.ID,
			Source:    evt.Source,
			Subject:   meta.Subject,
			Sender:    meta.Sender,
			Status:    evt.Status,
			CreatedAt: evt.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    result.Total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *EventsHandler) Get(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	evt, err := h.svc.GetEvent(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "event_not_found", "message": "Event with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get event"})
		return
	}

	c.JSON(http.StatusOK, evt)
}
