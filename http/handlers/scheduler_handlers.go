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

	// External Packages
	"github.com/go-chi/chi/v5"
)

type SchedulerService interface {
	GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error)
	Insert(ctx context.Context, task smodels.TaskModel) (string, error)
	Delete(ctx context.Context, taskID string) error
	Toggle(ctx context.Context, taskID string) error
}

type SchedulerHandler struct {
	schedulerService SchedulerService
}

func NewSchedulerHandler(schedulerService SchedulerService) *SchedulerHandler {
	return &SchedulerHandler{schedulerService: schedulerService}
}

func (h *SchedulerHandler) GetOne(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	task, err := h.schedulerService.GetOne(r.Context(), taskID)
	if err == nil {
		return task, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Insert(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	var task smodels.TaskModel
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		return nil, http.StatusBadRequest, errors.InvalidBodyErr(err)
	}
	if err := task.ValidateCreation(); err != nil {
		return nil, http.StatusBadRequest, errors.ValidationFailedErr(err)
	}

	taskID, err := h.schedulerService.Insert(r.Context(), task)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Created Task With ID : %s", taskID),
		}, http.StatusCreated, nil
	}
	return
}

func (h *SchedulerHandler) Delete(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	err = h.schedulerService.Delete(r.Context(), taskID)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Deleted Task With ID : %s", taskID),
		}, http.StatusOK, nil
	}
	return
}

func (h *SchedulerHandler) Toggle(w http.ResponseWriter, r *http.Request) (response any, status int, err error) {
	taskID := chi.URLParam(r, "taskId")
	err = h.schedulerService.Toggle(r.Context(), taskID)
	if err == nil {
		return map[string]interface{}{
			"message": fmt.Sprintf("Toggled Task With ID : %s", taskID),
		}, http.StatusOK, nil
	}
	return
}
