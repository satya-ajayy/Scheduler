package executer

import (
	// Go Internal Packages
	"context"
	"io"
	"math/rand"
	"time"

	// Local Packages
	smodels "scheduler/models"
	helpers "scheduler/utils/helpers"
	slack "scheduler/utils/slack"

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
	slack  slack.Sender
}

func NewExecutorService(logger *zap.Logger, task smodels.TaskModel, repo SchedulerRepo, slack slack.Sender) *ExecutorService {
	return &ExecutorService{
		logger: logger,
		task:   task,
		repo:   repo,
		slack:  slack,
	}
}

func (s *ExecutorService) Run() {
	attempts := s.task.NumberOfAttempts
	data := s.task.TaskData

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	baseDelay := 500 * time.Millisecond
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := helpers.CallAPI(ctx, data.URL, data.RequestType, data.RequestBody, data.Headers, data.QueryParams)
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.logger.Info("Task Executed Successfully",
				zap.String("taskId", s.task.ID), zap.Int("statusCode", resp.StatusCode))

			// Update task status on success
			if updateErr := s.repo.UpdateTaskStatus(ctx, s.task.ID, "", true); updateErr != nil {
				s.logger.Error("Failed To Update Task Status", zap.Error(updateErr))
			}
			return
		}

		s.logger.Warn("Task Execution Failed, Retrying",
			zap.String("taskId", s.task.ID),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)

		if attempt == attempts {
			exceptionMsg := ""
			if resp != nil {
				s.logger.Warn("API Call Failed",
					zap.String("url", data.URL),
					zap.String("status", resp.Status))
				exceptionMsg = resp.Status
			}
			if err != nil {
				exceptionMsg = err.Error()
			}

			s.logger.Error("Max Retry Attempts Reached, Task Failed", zap.String("taskId", s.task.ID))

			// Update task status on failure
			if updateErr := s.repo.UpdateTaskStatus(ctx, s.task.ID, exceptionMsg, false); updateErr != nil {
				s.logger.Error("Failed To Update Task Status", zap.Error(updateErr))
			}

			// Send slack alert
			if sendErr := s.slack.SendAlert(s.task, exceptionMsg); sendErr != nil {
				s.logger.Error("Error Sending Slack Alert", zap.Error(sendErr))
			}
			return
		}

		jitter := time.Duration(rand.Intn(300)) * time.Millisecond
		backoff := time.Duration(attempt) * baseDelay
		select {
		case <-time.After(backoff + jitter):
		case <-ctx.Done():
			return
		}
	}
}
