package scheduler

import (
	// Go Internal Packages
	"context"
	"fmt"
	"scheduler/config"
	"sync"
	"time"

	// Local Packages
	errors "scheduler/errors"
	smodels "scheduler/models"
	utils "scheduler/utils"

	// External Packages
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type SchedulerRepo interface {
	GetOne(ctx context.Context, taskID string) (smodels.TaskModel, error)
	GetActive(ctx context.Context, curUnix utils.Unix) ([]smodels.TaskModel, error)
	Insert(ctx context.Context, task smodels.TaskModel) error
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
	UpdateEnable(ctx context.Context, taskID string, enable bool) error
	Delete(ctx context.Context, taskID string) error
}

type SchedulerService struct {
	logger        *zap.Logger
	schedulerRepo SchedulerRepo
	config        config.Config
	cron          *cron.Cron
	tasks         map[string]cron.EntryID
	tasksMu       sync.Mutex
	timers        map[string]context.CancelFunc
	timersMu      sync.Mutex
}

func NewSchedulerService(schedulerRepo SchedulerRepo, logger *zap.Logger, config config.Config) *SchedulerService {
	cronObj := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
	tasksMap := make(map[string]cron.EntryID)
	timersMap := make(map[string]context.CancelFunc)
	return &SchedulerService{
		logger:        logger,
		schedulerRepo: schedulerRepo,
		config:        config,
		cron:          cronObj,
		tasks:         tasksMap,
		timers:        timersMap,
	}
}

func (s *SchedulerService) GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error) {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.E(errors.Invalid, "task not found with given id")
		}
		return nil, fmt.Errorf("failed to fetch task data: %w", err)
	}
	return &task, nil
}

func (s *SchedulerService) Insert(ctx context.Context, task smodels.TaskModel) (string, error) {
	task.ID = uuid.New().String()
	currentTime := utils.GetCurrentDateTime()
	task.CreatedAt = currentTime
	task.UpdatedAt = currentTime
	err := s.schedulerRepo.Insert(ctx, task)
	if err != nil {
		return "", fmt.Errorf("failed to insert task: %w", err)
	}
	s.ScheduleTask(task)
	return task.ID, nil
}

func (s *SchedulerService) Delete(ctx context.Context, taskID string) error {
	s.DiscardTaskNow(taskID)
	err := s.schedulerRepo.Delete(ctx, taskID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.E(errors.Invalid, "task not found with given id")
		}
		return fmt.Errorf("failed to delete task: %w", err)
	}
	return nil
}

func (s *SchedulerService) Toggle(ctx context.Context, taskID string) error {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if err != nil {
		return err
	}

	enable := !task.Enable
	err = s.schedulerRepo.UpdateEnable(ctx, taskID, enable)
	if err != nil {
		return fmt.Errorf("failed to update enabale status: %w", err)
	}

	alreadyExecuted := task.Status.IsAlreadyExecuted()
	if alreadyExecuted && !task.IsRecurEnabled {
		return nil
	}

	if enable {
		s.ScheduleTask(task)
	} else {
		s.DiscardTaskNow(taskID)
	}
	return nil
}
