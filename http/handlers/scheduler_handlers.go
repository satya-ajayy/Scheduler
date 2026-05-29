package handlers

import (
	// Go Internal Packages
	"context"
	"encoding/json"
	"net/http"

	// Local Packages
	errors "scheduler/errors"
	smodels "scheduler/models"

	// External Packages
	"github.com/go-chi/chi/v5"
)

type SchedulerService interface {
	GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error)
	GetActive(ctx context.Context) (*smodels.ActiveTasks, error)
	Insert(ctx context.Context, taskQP smodels.TaskQP) (string, error)
	Delete(ctx context.Context, taskID string) error
	Enable(ctx context.Context, taskID string) error
	Disable(ctx context.Context, taskID string) error
	ExecuteNow(ctx context.Context, taskID string) error
}

type SchedulerHandler struct {
	schedulerService SchedulerService
}

func NewSchedulerHandler(schedulerService SchedulerService) *SchedulerHandler {
	return &SchedulerHandler{schedulerService: schedulerService}
}

func (h *SchedulerHandler) GetOne(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	task, err := h.schedulerService.GetOne(r.Context(), taskID)
	if err == nil {
		return task, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Insert(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var taskQP smodels.TaskQP
	if err = json.NewDecoder(r.Body).Decode(&taskQP); err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	if err = taskQP.Validate(); err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}

	taskID, err := h.schedulerService.Insert(r.Context(), taskQP)
	if err == nil {
		return map[string]any{
			"message": "Task Created Successfully",
			"taskId":  taskID,
		}, http.StatusCreated, nil
	}
	return
}

func (h *SchedulerHandler) Delete(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.Delete(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Deleted Successfully",
			"taskId":  taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Enable(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.Enable(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Enabled Successfully",
			"taskId":  taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Disable(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.Disable(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Disabled Successfully",
			"taskId":  taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) GetActive(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	activeTasks, err := h.schedulerService.GetActive(r.Context())
	if err == nil {
		return activeTasks, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Execute(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.ExecuteNow(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Executed Successfully",
			"taskId":  taskID,
		}, http.StatusOK, nil
	}
	return
}
