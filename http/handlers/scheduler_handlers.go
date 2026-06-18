package handlers

import (
	// Go Internal Packages
	"context"
	"encoding/json"
	"net/http"

	// Local Packages
	errors "scheduler/errors"
	models "scheduler/models"

	// External Packages
	"github.com/go-chi/chi/v5"
)

type SchedulerService interface {
	GetOne(ctx context.Context, taskID string) (*models.Task, error)
	GetActive(ctx context.Context) (*models.ActiveList, error)
	Insert(ctx context.Context, taskQP models.CreateRequest) (string, error)
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

func (h *SchedulerHandler) GetOne(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("task_id")
	}

	task, err := h.schedulerService.GetOne(r.Context(), taskID)
	if err == nil {
		return task, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Insert(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var taskQP models.CreateRequest
	if err = json.NewDecoder(r.Body).Decode(&taskQP); err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	taskQP.Normalize()
	if err = taskQP.Validate(); err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}

	taskID, err := h.schedulerService.Insert(r.Context(), taskQP)
	if err == nil {
		return map[string]any{
			"message": "Task Created Successfully",
			"task_id": taskID,
		}, http.StatusCreated, nil
	}
	return
}

func (h *SchedulerHandler) Delete(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("task_id")
	}

	err = h.schedulerService.Delete(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Deleted Successfully",
			"task_id": taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Enable(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("task_id")
	}

	err = h.schedulerService.Enable(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Enabled Successfully",
			"task_id": taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Disable(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("task_id")
	}

	err = h.schedulerService.Disable(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Disabled Successfully",
			"task_id": taskID,
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) GetActive(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	activeTasks, err := h.schedulerService.GetActive(r.Context())
	if err == nil {
		return activeTasks, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Execute(_ http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("task_id")
	}

	err = h.schedulerService.ExecuteNow(r.Context(), taskID)
	if err == nil {
		return map[string]any{
			"message": "Task Executed Successfully",
			"task_id": taskID,
		}, http.StatusOK, nil
	}
	return
}
