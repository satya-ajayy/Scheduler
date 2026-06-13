package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	errors "scheduler/internal/errors"
	task "scheduler/internal/task"
	httpclient "scheduler/pkg/httpclient"
	notifier "scheduler/pkg/notifier"
	timex "scheduler/pkg/timex"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

type SchedulerRepo interface {
	GetOne(ctx context.Context, taskID string) (task.Task, error)
	GetActive(ctx context.Context, curUnix timex.Unix) ([]task.Task, error)
	Insert(ctx context.Context, task task.Task) error
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
	UpdateEnable(ctx context.Context, taskID string, enable bool) (bool, error)
	Delete(ctx context.Context, taskID string) error
}

type SchedulerService struct {
	ctx           context.Context
	logger        *zap.Logger
	schedulerRepo SchedulerRepo
	slack         notifier.Sender
	client        *httpclient.Client
	cron          *cron.Cron
	tasks         map[string]cron.EntryID
	tasksMu       sync.Mutex
	timers        map[timerKey]context.CancelFunc
	timersMu      sync.Mutex
}

func NewService(ctx context.Context, logger *zap.Logger, schedulerRepo SchedulerRepo, slack notifier.Sender, client *httpclient.Client) *SchedulerService {
	cronObj := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
	return &SchedulerService{
		ctx:           ctx,
		logger:        logger,
		schedulerRepo: schedulerRepo,
		slack:         slack,
		client:        client,
		cron:          cronObj,
		tasks:         make(map[string]cron.EntryID),
		timers:        make(map[timerKey]context.CancelFunc),
	}
}

func (s *SchedulerService) GetOne(ctx context.Context, taskID string) (*task.Task, error) {
	t, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errors.NewError(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task data: %w", err)
	}
	return &t, nil
}

func (s *SchedulerService) GetActive(ctx context.Context) (*task.ActiveList, error) {
	curUnix := timex.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active tasks: %w", err)
	}

	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.ID)
	}
	return &task.ActiveList{ActiveTasks: out}, nil
}

func (s *SchedulerService) Insert(ctx context.Context, taskQP task.CreateRequest) (string, error) {
	taskID := uuid.New().String()
	curTime := timex.GetCurrentDateTime()
	t, err := taskQP.ToTask(taskID, curTime)
	if err != nil {
		return "", fmt.Errorf("failed to build task: %w", err)
	}
	if err := s.schedulerRepo.Insert(ctx, t); err != nil {
		return "", fmt.Errorf("failed to insert task: %w", err)
	}
	s.scheduleTask(t)
	return t.ID, nil
}

func (s *SchedulerService) Delete(ctx context.Context, taskID string) error {
	err := s.schedulerRepo.Delete(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.NewError(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	s.discardTaskNow(taskID)
	return nil
}

func (s *SchedulerService) Enable(ctx context.Context, taskID string) error {
	t, err := s.GetOne(ctx, taskID)
	if err != nil {
		return err
	}

	updated, err := s.schedulerRepo.UpdateEnable(ctx, taskID, true)
	if err != nil {
		return fmt.Errorf("failed to update enable status: %w", err)
	}
	if !updated {
		s.logger.Info("Task Is Already Enabled", zap.String("taskId", taskID))
		return nil
	}

	alreadyExecuted := t.Status.IsAlreadyExecuted()
	if alreadyExecuted && !t.IsRecurEnabled {
		s.logger.Info("Non Recurring Task Already Executed, Skipping Reschedule",
			zap.String("taskId", taskID))
		return nil
	}

	curUnix := timex.CurrentUTCUnix()
	if curUnix > timex.Unix(t.EndUnix) {
		s.logger.Info("Task Already Expired", zap.String("taskId", taskID))
		return nil
	}

	s.scheduleTask(*t)
	return nil
}

func (s *SchedulerService) Disable(ctx context.Context, taskID string) error {
	if _, err := s.GetOne(ctx, taskID); err != nil {
		return err
	}

	updated, err := s.schedulerRepo.UpdateEnable(ctx, taskID, false)
	if err != nil {
		return fmt.Errorf("failed to update enable status: %w", err)
	}
	if !updated {
		s.logger.Info("Task Is Already Disabled", zap.String("taskId", taskID))
		return nil
	}

	s.discardTaskNow(taskID)
	return nil
}

func (s *SchedulerService) ExecuteNow(ctx context.Context, taskID string) error {
	t, err := s.GetOne(ctx, taskID)
	if err != nil {
		return err
	}

	curUnix := timex.CurrentUTCUnix()
	if curUnix > timex.Unix(t.EndUnix) {
		s.logger.Info("Task Already Expired", zap.String("taskId", taskID))
		return nil
	}

	s.executeTaskNow(*t)
	return nil
}
