package tasks

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) List(c *gin.Context) {
	orgID := c.GetString("org_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	status := c.Query("status")

	result, err := h.repo.List(c.Request.Context(), ListParams{
		OrgID:  orgID,
		Status: status,
		Page:   page,
		Limit:  perPage,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to list tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     result.Tasks,
		"total":    result.Total,
		"page":     page,
		"per_page": perPage,
	})
}

func (h *Handler) Get(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	task, err := h.repo.GetByID(c.Request.Context(), id, orgID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task_not_found", "message": "Task with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to get task"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *Handler) Update(c *gin.Context) {
	orgID := c.GetString("org_id")
	id := c.Param("id")

	var req struct {
		Status     *string `json:"status"`
		AssigneeID *string `json:"assignee_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "Invalid request body"})
		return
	}

	task, err := h.repo.Update(c.Request.Context(), id, orgID, UpdateParams{
		Status:     req.Status,
		AssigneeID: req.AssigneeID,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task_not_found", "message": "Task with the specified ID was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Failed to update task"})
		return
	}

	c.JSON(http.StatusOK, task)
}
