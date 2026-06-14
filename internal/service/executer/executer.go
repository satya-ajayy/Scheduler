package executer

import (
	"context"
	"io"
	"math/rand"
	"time"

	task "scheduler/internal/task"
	httpclient "scheduler/pkg/httpclient"
	notifier "scheduler/pkg/notifier"

	"go.uber.org/zap"
)

type SchedulerRepo interface {
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
}

type ExecutorService struct {
	ctx    context.Context
	logger *zap.Logger
	task   task.Task
	repo   SchedulerRepo
	slack  notifier.Sender
	client *httpclient.Client
}

func NewExecutorService(ctx context.Context, logger *zap.Logger, task task.Task, repo SchedulerRepo, slack notifier.Sender, client *httpclient.Client) *ExecutorService {
	return &ExecutorService{
		ctx:    ctx,
		logger: logger,
		task:   task,
		repo:   repo,
		slack:  slack,
		client: client,
	}
}

func (s *ExecutorService) Run() {
	attempts := s.task.NumberOfAttempts
	data := s.task.TaskData

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	baseDelay := 500 * time.Millisecond
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := s.client.Do(ctx, httpclient.Request{
			URL:         data.URL,
			Method:      data.RequestType,
			Headers:     data.Headers,
			QueryParams: data.QueryParams,
			Body:        data.RequestBody,
		})
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.logger.Info("Task Executed Successfully",
				zap.String("taskId", s.task.ID), zap.Int("statusCode", resp.StatusCode))

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
				s.logger.Warn("API Call Failed", zap.String("url", data.URL), zap.String("status", resp.Status))
				exceptionMsg = resp.Status
			}
			if err != nil {
				exceptionMsg = err.Error()
			}

			s.logger.Error("Max Retry Attempts Reached, Task Failed", zap.String("taskId", s.task.ID))

			if updateErr := s.repo.UpdateTaskStatus(ctx, s.task.ID, exceptionMsg, false); updateErr != nil {
				s.logger.Error("Failed To Update Task Status", zap.Error(updateErr))
			}

			if sendErr := s.slack.SendAlert(ctx, s.task, exceptionMsg); sendErr != nil {
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
