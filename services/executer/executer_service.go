package executer

import (
	// Go Internal Packages
	"context"
	"fmt"
	"math/rand"
	"time"

	// Local Packages
	smodels "scheduler/models"
	utils "scheduler/utils"

	// External Packages
	"go.uber.org/zap"
)

type SchedulerRepo interface {
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
}

type ExecutorService struct {
	logger *zap.Logger
	task   smodels.TaskModel
	repo   SchedulerRepo
}

func NewExecutorService(logger *zap.Logger, task smodels.TaskModel, repo SchedulerRepo) *ExecutorService {
	return &ExecutorService{logger: logger, task: task, repo: repo}
}

func (s *ExecutorService) Run() {
	attempts := s.task.NumberOfAttempts
	data := s.task.TaskData

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	baseDelay := 500 * time.Millisecond
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := utils.CallAPI(data.URL, data.RequestType, data.RequestBody, data.Headers, data.QueryParams)

		if err == nil && resp != nil && (resp.StatusCode >= 200 && resp.StatusCode < 300) {
			s.logger.Info("Task executed successfully",
				zap.String("task_id", s.task.ID),
				zap.Int("status_code", resp.StatusCode),
			)
			if updateErr := s.repo.UpdateTaskStatus(ctx, s.task.ID, "", true); updateErr != nil {
				s.logger.Error("Failed to update task status", zap.Error(updateErr))
			}
			return
		}

		s.logger.Warn("Task execution failed, retrying", zap.String("task_id", s.task.ID),
			zap.Int("attempt", attempt), zap.Error(err),
		)

		if attempt == attempts {
			exceptionMsg := ""
			if resp != nil {
				fmt.Println("API call to " + data.URL + " is failed with status: " + resp.Status)
				exceptionMsg = resp.Status
			}
			s.logger.Error("Max retry attempts reached, task failed", zap.String("task_id", s.task.ID))
			if err != nil {
				exceptionMsg = err.Error()
			}
			if updateErr := s.repo.UpdateTaskStatus(ctx, s.task.ID, exceptionMsg, false); updateErr != nil {
				s.logger.Error("Failed to update task status", zap.Error(updateErr))
			}
			return
		}

		jitter := time.Duration(rand.Intn(300)) * time.Millisecond
		backoff := time.Duration(attempt) * baseDelay
		time.Sleep(backoff + jitter)
	}
}
