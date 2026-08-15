package handler

import (
	"log/slog"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"andreypostal.com/cubby/server/pkg/modules/tasks/dto"
	"andreypostal.com/cubby/server/pkg/modules/tasks/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type TaskHandler struct {
	taskService service.TaskService
	validate    *validator.Validate
}

func NewTaskHandler(taskService service.TaskService, validate *validator.Validate) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		validate:    validate,
	}
}

func (h *TaskHandler) GetBoard(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.taskService.GetBoard(profile.Workspace.ID)
	if err != nil {
		logger.Error("Failed to fetch task board", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve task board", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) UpdateBoard(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.UpdateBoardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.taskService.UpdateBoard(profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to update task board", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Defaulting to the running tasks keeps the common case — "what am I on the
	// hook for?" — a plain GET with no query at all.
	filter := c.DefaultQuery("status", service.FilterActive)
	if !service.IsValidListFilter(filter) {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "status must be one of active, stopped or all", nil))
		return
	}

	resp, err := h.taskService.List(profile.Workspace.ID, filter)
	if err != nil {
		logger.Error("Failed to fetch tasks for workspace", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve tasks from workspace", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.taskService.Create(profile.Workspace.ID, &req)
	if err != nil {
		logger.Warn("error while creating new task", slog.String("reason", err.Error()))
		respondServiceError(c, "error while creating new task", err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	resp, err := h.taskService.Get(taskID, profile.Workspace.ID)
	if err != nil {
		respondServiceError(c, "failed to fetch task", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) UpdateTask(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	var req dto.UpdateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.taskService.Update(taskID, profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to update task", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) DeleteTask(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	if err := h.taskService.Delete(taskID, profile.Workspace.ID); err != nil {
		respondServiceError(c, "failed to remove task", err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *TaskHandler) StopTask(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	resp, err := h.taskService.Stop(taskID, profile.Workspace.ID)
	if err != nil {
		respondServiceError(c, "failed to stop task", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) ResumeTask(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	// The body is optional: no body means "resume from today".
	var req dto.ResumeTaskReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
			return
		}

		if err := h.validate.Struct(req); err != nil {
			core.RespondError(c, core.ValidationError(err))
			return
		}
	}

	resp, err := h.taskService.Resume(taskID, profile.Workspace.ID, &req)
	if err != nil {
		respondServiceError(c, "failed to resume task", err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *TaskHandler) Day(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.taskService.Day(profile.Workspace.ID, c.Query("date"))
	if err != nil {
		respondServiceError(c, "failed to fetch day view", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) Range(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.taskService.Range(profile.Workspace.ID, c.Query("from"), c.Query("to"))
	if err != nil {
		respondServiceError(c, "failed to fetch range view", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) MarkDay(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	day, err := core.ParseDate(c.Param("date"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid date, expected format YYYY-MM-DD", nil))
		return
	}

	var req dto.MarkDayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.taskService.MarkDay(taskID, profile.Workspace.ID, day, &req)
	if err != nil {
		respondServiceError(c, "failed to mark task day", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *TaskHandler) ClearDay(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	taskID, err := core.FromStringToUint(c.Param("taskID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid task id", nil))
		return
	}

	day, err := core.ParseDate(c.Param("date"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid date, expected format YYYY-MM-DD", nil))
		return
	}

	resp, err := h.taskService.ClearDay(taskID, profile.Workspace.ID, day)
	if err != nil {
		respondServiceError(c, "failed to clear task day", err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
