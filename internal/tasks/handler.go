package tasks

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

type taskListItem struct {
	ID          string  `json:"id" example:"task_001"`
	Title       string  `json:"title" example:"Prepare contract review"`
	Description *string `json:"description,omitempty" example:"Review the updated contract terms from partner"`
	Status      string  `json:"status" example:"pending" enums:"pending,in_progress,completed,cancelled"`
	AssigneeID  *string `json:"assignee_id,omitempty" example:"user_001"`
	DueDate     *string `json:"due_date,omitempty" example:"2026-06-17T00:00:00Z"`
	CreatedAt   string  `json:"created_at" example:"2026-06-10T12:02:00Z"`
}

// List godoc
// @Summary      List tasks
// @Description  List tasks for the authenticated user's organization with optional status filter.
// @Tags         Tasks
// @Accept       json
// @Produce      json
// @Param        status    query  string  false  "Filter by status (pending, in_progress, completed, cancelled)"
// @Param        page      query  int     false  "Page number (default 1)"
// @Param        per_page  query  int     false  "Items per page (default 20)"
// @Success      200       {object}  object{data=[]taskListItem,total=int,page=int,per_page=int}
// @Security     Bearer
// @Router       /tasks [get]
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

	items := make([]taskListItem, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		item := taskListItem{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Status:      t.Status,
			AssigneeID:  t.AssigneeID,
			CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		}
		if t.DueDate != nil {
			s := t.DueDate.Format(time.RFC3339)
			item.DueDate = &s
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    result.Total,
		"page":     page,
		"per_page": perPage,
	})
}

// Get godoc
// @Summary      Get task
// @Description  Get a single task by ID.
// @Tags         Tasks
// @Produce      json
// @Param        id   path  string  true  "Task ID"
// @Success      200  {object}  object{id=string,title=string,description=string,status=string,due_date=string}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /tasks/{id} [get]
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

// Update godoc
// @Summary      Update task
// @Description  Update a task's status or assignee.
// @Tags         Tasks
// @Accept       json
// @Produce      json
// @Param        id       path  string                                           true  "Task ID"
// @Param        request  body  object{status=string,assignee_id=string}  true  "Fields to update"
// @Success      200  {object}  object{id=string,title=string,status=string}
// @Failure      400  {object}  object{error=string,message=string}
// @Failure      404  {object}  object{error=string,message=string}
// @Security     Bearer
// @Router       /tasks/{id} [patch]
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
