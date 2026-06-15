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
	ID        string `json:"id" example:"evt_001"`
	Source    string `json:"source" example:"email"`
	Subject   string `json:"subject" example:"Contract Review Meeting"`
	Sender    string `json:"sender" example:"partner@company.com"`
	Status    string `json:"status" example:"completed"`
	CreatedAt string `json:"created_at" example:"2026-06-10T12:00:00Z"`
}

// List godoc
// @Summary      List events
// @Description  List incoming events for the authenticated user's organization.
// @Tags         Events
// @Accept       json
// @Produce      json
// @Param        status    query  string  false  "Filter by status (new, processing, completed, failed)"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 20)"
// @Success      200       {object}  object{data=[]eventListItem,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /events [get]
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

// Get godoc
// @Summary      Get event
// @Description  Get a single event by ID (without analysis/logs nesting — use GET /events/:id for the full detail view).
// @Tags         Events
// @Produce      json
// @Param        id   path  string  true  "Event ID"
// @Success      200  {object}  object{id=string,source=string,status=string,metadata=object}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /events/{id} [get]
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

// Reanalyze godoc
// @Summary      Re-analyze event
// @Description  Reset event status to 'new' so the worker re-processes it through AI analysis and action execution.
// @Tags         Events
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "Event ID"
// @Success      200  {object}  object{status=string}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /events/{id}/reanalyze [post]
func (h *EventsHandler) Reanalyze(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	if err := h.svc.Reanalyze(c.Request.Context(), id, orgID); err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "event_not_found", "message": "Event with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to reanalyze event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analysis_id": id + "_reanalysis",
		"status":      "processing",
	})
}
