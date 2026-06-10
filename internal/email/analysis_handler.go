package email

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nurik/Dev/repos/mengu-backend/internal/actions"
	"github.com/nurik/Dev/repos/mengu-backend/internal/ai"
)

type EventDetailHandler struct {
	eventRepo    *Repository
	analysisRepo *ai.Repository
	actionsRepo  *actions.Repository
}

func NewEventDetailHandler(eventRepo *Repository, analysisRepo *ai.Repository, actionsRepo *actions.Repository) *EventDetailHandler {
	return &EventDetailHandler{
		eventRepo:    eventRepo,
		analysisRepo: analysisRepo,
		actionsRepo:  actionsRepo,
	}
}

// GetWithDetails godoc
// @Summary      Get event with analysis and logs
// @Description  Get a single event with its associated AI analysis and action logs.
// @Tags         Events
// @Produce      json
// @Param        id   path  string  true  "Event ID"
// @Success      200  {object}  object{event=object,analysis=object,action_logs=array}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /events/{id} [get]
func (h *EventDetailHandler) GetWithDetails(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	evt, err := h.eventRepo.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "event_not_found", "message": "Event with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get event"})
		return
	}

	analysis, _ := h.analysisRepo.GetLatestByEventID(c.Request.Context(), id, orgID)

	logsResult, _ := h.actionsRepo.ListLogs(c.Request.Context(), actions.LogListParams{
		EventID: id,
		OrgID:   orgID,
		Page:    1,
		Limit:   100,
	})

	resp := gin.H{
		"event": evt,
	}
	if analysis != nil {
		resp["analysis"] = analysis
	}
	if logsResult != nil {
		resp["action_logs"] = logsResult.Logs
	}

	c.JSON(http.StatusOK, resp)
}

// GetAnalysis godoc
// @Summary      Get AI analysis for event
// @Description  Get the AI analysis (intent, confidence, actions) for a specific event.
// @Tags         Events
// @Produce      json
// @Param        id   path  string  true  "Event ID"
// @Success      200  {object}  object{id=string,event_id=string,intent=string,confidence=number,actions=array}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /events/{id}/analysis [get]
func (h *EventDetailHandler) GetAnalysis(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	analysis, err := h.analysisRepo.GetLatestByEventID(c.Request.Context(), id, orgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "analysis_not_found", "message": "No AI analysis found for this event"})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// GetLogs godoc
// @Summary      Get action logs for event
// @Description  Get action execution logs (one entry per action) for a specific event.
// @Tags         Events
// @Produce      json
// @Param        id        path  string  true  "Event ID"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 20)"
// @Success      200       {object}  object{data=array,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /events/{id}/logs [get]
func (h *EventDetailHandler) GetLogs(c *gin.Context) {
	orgID := c.GetString("org_id")
	eventID := c.Param("id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	result, err := h.actionsRepo.ListLogs(c.Request.Context(), actions.LogListParams{
		EventID: eventID,
		OrgID:   orgID,
		Page:    page,
		Limit:   perPage,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list action logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     result.Logs,
		"total":    result.Total,
		"page":     page,
		"per_page": perPage,
	})
}

// GetCalendarEvents godoc
// @Summary      Get calendar events for event
// @Description  List calendar events created from an event (data sourced from action logs).
// @Tags         Events
// @Produce      json
// @Param        id   path  string  true  "Event ID"
// @Success      200  {object}  object{data=[]object,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /events/{id}/calendar-events [get]
func (h *EventDetailHandler) GetCalendarEvents(c *gin.Context) {
	orgID := c.GetString("org_id")
	eventID := c.Param("id")

	logsResult, err := h.actionsRepo.ListLogs(c.Request.Context(), actions.LogListParams{
		EventID: eventID,
		OrgID:   orgID,
		Page:    1,
		Limit:   100,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list calendar events"})
		return
	}

	events := make([]gin.H, 0)
	for _, log := range logsResult.Logs {
		if log.ActionType == "schedule_meeting" {
			var payload struct {
				Title    string `json:"title"`
				Datetime string `json:"datetime"`
			}
			if len(log.Payload) > 0 {
				json.Unmarshal(log.Payload, &payload)
			}
			events = append(events, gin.H{
				"title":          payload.Title,
				"datetime":       payload.Datetime,
				"google_event_id": "google_cal_event_001",
				"status":         "created",
				"created_at":     log.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     events,
		"total":    len(events),
		"page":     1,
		"per_page": 20,
	})
}
