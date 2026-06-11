package scheduler

import (
	// Go Internal Packages
	"context"
	"fmt"
	"sync"
	"time"

	// Local Packages
	errors "scheduler/errors"
	smodels "scheduler/models"
	helpers "scheduler/utils/helpers"
	slack "scheduler/utils/slack"

	// External Packages
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

type SchedulerRepo interface {
	GetOne(ctx context.Context, taskID string) (smodels.TaskModel, error)
	GetActive(ctx context.Context, curUnix helpers.Unix) ([]smodels.TaskModel, error)
	Insert(ctx context.Context, task smodels.TaskModel) error
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
	UpdateEnable(ctx context.Context, taskID string, enable bool) (bool, error)
	Delete(ctx context.Context, taskID string) error
}

type SchedulerService struct {
	logger        *zap.Logger
	schedulerRepo SchedulerRepo
	slack         slack.Sender
	cron          *cron.Cron
	tasks         map[string]cron.EntryID
	tasksMu       sync.Mutex
	timers        map[string]context.CancelFunc
	timersMu      sync.Mutex
}

func NewSchedulerService(logger *zap.Logger, schedulerRepo SchedulerRepo, slack slack.Sender) *SchedulerService {
	cronObj := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
	return &SchedulerService{
		logger:        logger,
		schedulerRepo: schedulerRepo,
		slack:         slack,
		cron:          cronObj,
		tasks:         make(map[string]cron.EntryID),
		timers:        make(map[string]context.CancelFunc),
	}
}

func (s *SchedulerService) GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error) {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errors.E(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task data: %w", err)
	}
	return &task, nil
}

func (s *SchedulerService) GetActive(ctx context.Context) (*smodels.ActiveTasks, error) {
	curUnix := helpers.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active tasks: %w", err)
	}

	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.ID)
	}
	return &smodels.ActiveTasks{ActiveTasks: out}, nil
}

func (s *SchedulerService) Insert(ctx context.Context, taskQP smodels.TaskQP) (string, error) {
	taskID := uuid.New().String()
	curTime := helpers.GetCurrentDateTime()
	task := taskQP.ToTaskModel(taskID, curTime)
	if err := s.schedulerRepo.Insert(ctx, task); err != nil {
		return "", fmt.Errorf("failed to insert task: %w", err)
	}
	s.ScheduleTask(task)
	return task.ID, nil
}

func (s *SchedulerService) Delete(ctx context.Context, taskID string) error {
	err := s.schedulerRepo.Delete(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	s.DiscardTaskNow(taskID)
	return nil
}

func (s *SchedulerService) Enable(ctx context.Context, taskID string) error {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to fetch task data: %w", err)
	}

	updated, err := s.schedulerRepo.UpdateEnable(ctx, taskID, true)
	if err != nil {
		return fmt.Errorf("failed to update enable status: %w", err)
	}
	if !updated {
		s.logger.Info("Task Is Already Enabled", zap.String("taskId", taskID))
		return nil
	}

	alreadyExecuted := task.Status.IsAlreadyExecuted()
	if alreadyExecuted && !task.IsRecurEnabled {
		s.logger.Info("Non Recurring Task Already Executed, Skipping Reschedule",
			zap.String("taskId", taskID))
		return nil
	}

	curUnix := helpers.CurrentUTCUnix()
	if curUnix > helpers.Unix(task.EndUnix) {
		s.logger.Info("Task Already Expired", zap.String("taskId", taskID))
		return nil
	}

	s.ScheduleTask(task)
	return nil
}

func (s *SchedulerService) Disable(ctx context.Context, taskID string) error {
	_, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to fetch task data: %w", err)
	}

	updated, err := s.schedulerRepo.UpdateEnable(ctx, taskID, false)
	if err != nil {
		return fmt.Errorf("failed to update enable status: %w", err)
	}
	if !updated {
		s.logger.Info("Task Is Already Disabled", zap.String("taskId", taskID))
		return nil
	}

	s.DiscardTaskNow(taskID)
	return nil
}

func (s *SchedulerService) ExecuteNow(ctx context.Context, taskID string) error {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.NotFound, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to fetch task data: %w", err)
	}

	curUnix := helpers.CurrentUTCUnix()
	if curUnix > helpers.Unix(task.EndUnix) {
		s.logger.Info("Task Already Expired", zap.String("taskId", taskID))
		return nil
	}

	s.ExecuteTaskNow(task)
	return nil
}
