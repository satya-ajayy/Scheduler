package handlers

import (
	// Go Internal Packages
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	// Local Packages
	errors "scheduler/errors"
	smodels "scheduler/models"
)

type SchedulerService interface {
	GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error)
	GetActive(ctx context.Context) (*smodels.ActiveTasks, error)
	Insert(ctx context.Context, taskQP smodels.TaskQP) (string, error)
	Delete(ctx context.Context, taskID string) error
	Toggle(ctx context.Context, taskID string) error
	ExecuteNow(ctx context.Context, taskID string) error
}

type SchedulerHandler struct {
	schedulerService SchedulerService
}

func NewSchedulerHandler(schedulerService SchedulerService) *SchedulerHandler {
	return &SchedulerHandler{schedulerService: schedulerService}
}

func (h *SchedulerHandler) GetOne(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := r.URL.Query().Get("taskId")
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
		return map[string]interface{}{
			"message": fmt.Sprintf("Created Task With ID: %s", taskID),
		}, http.StatusCreated, nil
	}
	return
}

func (h *SchedulerHandler) Delete(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := r.URL.Query().Get("taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.Delete(r.Context(), taskID)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Deleted Task With ID: %s", taskID),
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Toggle(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := r.URL.Query().Get("taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.Toggle(r.Context(), taskID)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Toggled Task With ID: %s", taskID),
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
	taskID := r.URL.Query().Get("taskId")
	if taskID == "" {
		return nil, http.StatusBadRequest, errors.EmptyParamErr("taskId")
	}

	err = h.schedulerService.ExecuteNow(r.Context(), taskID)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Executed Task With ID: %s", taskID),
		}, http.StatusOK, nil
	}
	return
}
