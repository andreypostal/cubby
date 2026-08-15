package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/dto"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/service"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const dateLayout = "2006-01-02"

// RoutineHandler handles routine goals routes.
type RoutineHandler struct {
	routineService service.RoutineService
	validate       *validator.Validate
}

// NewRoutineHandler returns a RoutineHandler.
func NewRoutineHandler(routineService service.RoutineService, validate *validator.Validate) *RoutineHandler {
	return &RoutineHandler{
		routineService: routineService,
		validate:       validate,
	}
}

func (h *RoutineHandler) List(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	resp, err := h.routineService.ListForWorkspace(profile.Workspace.ID)
	if err != nil {
		logger.Error("Failed to fetch routines for workspace", slog.String("error", err.Error()))
		core.RespondError(c, core.NewError(http.StatusInternalServerError, "failed to retrieve routines from workspace", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RoutineHandler) CreateRoutine(c *gin.Context) {
	logger := middleware.GetLogger(c)

	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var req dto.CreateRoutineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.routineService.Create(profile.Workspace.ID, &req)
	if err != nil {
		logger.Warn("error while creating new routine", slog.String("reason", err.Error()))
		core.RespondError(c, core.NewError(http.StatusBadRequest, "error while creating new routine", nil))
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *RoutineHandler) GetRoutine(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	resp, err := h.routineService.Get(routineID, profile.Workspace.ID)
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to fetch routine", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RoutineHandler) UpdateRoutine(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	var req dto.UpdateRoutineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	if err := h.validate.Struct(req); err != nil {
		core.RespondError(c, core.ValidationError(err))
		return
	}

	resp, err := h.routineService.Update(routineID, profile.Workspace.ID, &req)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to update routine", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RoutineHandler) DeleteRoutine(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	if err := h.routineService.Delete(routineID, profile.Workspace.ID); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "failed to remove routine", nil))
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *RoutineHandler) Month(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	now := time.Now()

	year := now.Year()
	if raw := c.Query("year"); raw != "" {
		year, err = strconv.Atoi(raw)
		if err != nil {
			core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid year", nil))
			return
		}
	}

	month := int(now.Month())
	if raw := c.Query("month"); raw != "" {
		month, err = strconv.Atoi(raw)
		if err != nil || month < 1 || month > 12 {
			core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid month", nil))
			return
		}
	}

	resp, err := h.routineService.Month(routineID, profile.Workspace.ID, year, time.Month(month))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusForbidden, "failed to fetch routine month", nil))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RoutineHandler) MarkDay(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	day, err := time.Parse(dateLayout, c.Param("date"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid date, expected format YYYY-MM-DD", nil))
		return
	}

	var req dto.MarkDayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, err.Error(), nil))
		return
	}

	resp, err := h.routineService.MarkDay(routineID, profile.Workspace.ID, day, req.Achieved)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to mark routine day", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *RoutineHandler) UnmarkDay(c *gin.Context) {
	profile, err := fetchProfile(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	routineID, err := core.FromStringToUint(c.Param("routineID"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid routine id", nil))
		return
	}

	day, err := time.Parse(dateLayout, c.Param("date"))
	if err != nil {
		core.RespondError(c, core.NewError(http.StatusBadRequest, "invalid date, expected format YYYY-MM-DD", nil))
		return
	}

	resp, err := h.routineService.UnmarkDay(routineID, profile.Workspace.ID, day)
	if err != nil {
		coreErr := core.NewError(http.StatusBadRequest, "failed to unmark routine day", map[string]any{
			"reason": err.Error(),
		})
		core.RespondError(c, coreErr)
		return
	}

	c.JSON(http.StatusOK, resp)
}
