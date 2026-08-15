package service

import (
	"errors"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/dto"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/models"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/repository"
	"gorm.io/gorm"
)

const dateLayout = "2006-01-02"

const (
	StatusDisabled = "disabled"
	StatusAchieved = "achieved"
	StatusFailed   = "failed"
	StatusWaiting  = "waiting"
	StatusFuture   = "future"
)

type RoutineService interface {
	Create(workspaceID uint, req *dto.CreateRoutineReq) (*dto.RoutinePayload, error)
	Get(routineID, workspaceID uint) (*dto.RoutinePayload, error)
	Update(routineID, workspaceID uint, req *dto.UpdateRoutineReq) (*dto.RoutinePayload, error)
	Delete(routineID, workspaceID uint) error
	ListForWorkspace(workspaceID uint) (*dto.WorkspaceRoutinesResp, error)

	Month(routineID, workspaceID uint, year int, month time.Month) (*dto.RoutineMonthResp, error)
	MarkDay(routineID, workspaceID uint, day time.Time, achieved bool) (*dto.MarkDayResp, error)
	UnmarkDay(routineID, workspaceID uint, day time.Time) (*dto.RoutinePayload, error)
}

type rs struct {
	routineRepo repository.RoutineRepository
	dayRepo     repository.RoutineDayRepository
}

func NewRoutineService(routineRepo repository.RoutineRepository, dayRepo repository.RoutineDayRepository) RoutineService {
	return &rs{routineRepo: routineRepo, dayRepo: dayRepo}
}

func (s *rs) Create(workspaceID uint, req *dto.CreateRoutineReq) (*dto.RoutinePayload, error) {
	routine := &models.Routine{
		Name:        req.Name,
		Description: req.Description,
		Pinned:      req.Pinned,
		Active:      true,

		OnMonday:    req.OnMonday,
		OnTuesday:   req.OnTuesday,
		OnWednesday: req.OnWednesday,
		OnThursday:  req.OnThursday,
		OnFriday:    req.OnFriday,
		OnSaturday:  req.OnSaturday,
		OnSunday:    req.OnSunday,

		WorkspaceID: workspaceID,
	}

	if err := s.routineRepo.Create(routine); err != nil {
		return nil, err
	}

	return toRoutineResponse(routine), nil
}

func (s *rs) Get(routineID, workspaceID uint) (*dto.RoutinePayload, error) {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	return toRoutineResponse(routine), nil
}

func (s *rs) Update(routineID, workspaceID uint, req *dto.UpdateRoutineReq) (*dto.RoutinePayload, error) {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	routine.Name = req.Name
	routine.Description = req.Description
	routine.Pinned = req.Pinned
	routine.Active = req.Active

	routine.OnMonday = req.OnMonday
	routine.OnTuesday = req.OnTuesday
	routine.OnWednesday = req.OnWednesday
	routine.OnThursday = req.OnThursday
	routine.OnFriday = req.OnFriday
	routine.OnSaturday = req.OnSaturday
	routine.OnSunday = req.OnSunday

	if err := s.routineRepo.Update(routine); err != nil {
		return nil, err
	}

	return toRoutineResponse(routine), nil
}

func (s *rs) Delete(routineID, workspaceID uint) error {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return core.ErrNotAllowed
	}

	return s.routineRepo.Delete(routineID)
}

func (s *rs) ListForWorkspace(workspaceID uint) (*dto.WorkspaceRoutinesResp, error) {
	routines, err := s.routineRepo.ListForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	out := make([]dto.RoutinePayload, len(routines))
	for i, r := range routines {
		out[i] = *toRoutineResponse(r)
	}

	return &dto.WorkspaceRoutinesResp{Routines: out}, nil
}

func (s *rs) Month(routineID, workspaceID uint, year int, month time.Month) (*dto.RoutineMonthResp, error) {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	registered, err := s.dayRepo.ListForRoutineInRange(routineID, start, end)
	if err != nil {
		return nil, err
	}

	byDay := make(map[string]*models.RoutineDay, len(registered))
	for _, d := range registered {
		byDay[d.Day.Format(dateLayout)] = d
	}

	today := normalizeDate(time.Now())

	days := make([]dto.DayStatusPayload, 0, 31)
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		days = append(days, dto.DayStatusPayload{
			Day:    d.Format(dateLayout),
			Status: dayStatus(routine, d, today, byDay[d.Format(dateLayout)]),
		})
	}

	return &dto.RoutineMonthResp{
		RoutineID: routine.ID,
		Year:      year,
		Month:     int(month),
		Days:      days,
	}, nil
}

func (s *rs) MarkDay(routineID, workspaceID uint, day time.Time, achieved bool) (*dto.MarkDayResp, error) {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	day = normalizeDate(day)
	if day.After(normalizeDate(time.Now())) {
		return nil, errors.New("cannot change status for a future day")
	}

	if !routine.IsScheduledFor(day) {
		return nil, errors.New("day is not part of the routine schedule")
	}

	existing, err := s.dayRepo.FindByRoutineAndDay(routineID, day)
	if err != nil && !errors.Is(err, core.ErrResourceNotFound) {
		return nil, err
	}

	delta := 1
	if !achieved {
		delta = -1
	}

	if existing == nil || existing.DeletedAt.Valid {
		if existing == nil {
			existing = &models.RoutineDay{RoutineID: routineID, Day: day, Result: achieved}
			if err := s.dayRepo.Create(existing); err != nil {
				return nil, err
			}
		} else {
			existing.Result = achieved
			existing.DeletedAt = gorm.DeletedAt{}
			if err := s.dayRepo.Update(existing); err != nil {
				return nil, err
			}
		}
	} else {
		if existing.Result == achieved {
			return nil, errors.New("day already has this status")
		}
		existing.Result = achieved
		if err := s.dayRepo.Update(existing); err != nil {
			return nil, err
		}
		delta *= 2
	}

	routine.Points += delta
	if err := s.routineRepo.Update(routine); err != nil {
		return nil, err
	}

	status := StatusFailed
	if achieved {
		status = StatusAchieved
	}

	return &dto.MarkDayResp{
		Routine: *toRoutineResponse(routine),
		Day:     dto.DayStatusPayload{Day: day.Format(dateLayout), Status: status},
	}, nil
}

func (s *rs) UnmarkDay(routineID, workspaceID uint, day time.Time) (*dto.RoutinePayload, error) {
	routine, err := s.routineRepo.FindByID(routineID)
	if err != nil {
		return nil, normalizeError(err)
	}

	if routine.WorkspaceID != workspaceID {
		return nil, core.ErrNotAllowed
	}

	day = normalizeDate(day)
	if day.After(normalizeDate(time.Now())) {
		return nil, errors.New("cannot change status for a future day")
	}

	existing, err := s.dayRepo.FindByRoutineAndDay(routineID, day)
	if err != nil {
		return nil, normalizeError(err)
	}

	if err := s.dayRepo.Delete(existing.ID); err != nil {
		return nil, err
	}

	if existing.Result {
		routine.Points--
	} else {
		routine.Points++
	}

	if err := s.routineRepo.Update(routine); err != nil {
		return nil, err
	}

	return toRoutineResponse(routine), nil
}

func normalizeError(err error) error {
	if errors.Is(err, core.ErrEntityNotFound) || errors.Is(err, core.ErrResourceNotFound) {
		return core.ErrNotAllowed
	}
	return err
}

func normalizeDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func dayStatus(routine *models.Routine, day, today time.Time, reg *models.RoutineDay) string {
	if !routine.IsScheduledFor(day) {
		return StatusDisabled
	}

	if day.After(today) {
		return StatusFuture
	}

	if reg == nil {
		return StatusWaiting
	}

	if reg.Result {
		return StatusAchieved
	}
	return StatusFailed
}

func toRoutineResponse(r *models.Routine) *dto.RoutinePayload {
	return &dto.RoutinePayload{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Pinned:      r.Pinned,
		Active:      r.Active,
		Points:      r.Points,
		CreatedAt:   r.CreatedAt,

		OnMonday:    r.OnMonday,
		OnTuesday:   r.OnTuesday,
		OnWednesday: r.OnWednesday,
		OnThursday:  r.OnThursday,
		OnFriday:    r.OnFriday,
		OnSaturday:  r.OnSaturday,
		OnSunday:    r.OnSunday,
	}
}
