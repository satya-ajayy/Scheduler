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
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type SchedulerRepo interface {
	GetOne(ctx context.Context, taskID string) (smodels.TaskModel, error)
	GetActive(ctx context.Context, curUnix helpers.Unix) ([]smodels.TaskModel, error)
	Insert(ctx context.Context, task smodels.TaskModel) error
	UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error
	UpdateEnable(ctx context.Context, taskID string, enable bool) error
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

func NewSchedulerService(schedulerRepo SchedulerRepo, logger *zap.Logger, slack slack.Sender) *SchedulerService {
	cronObj := cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC))
	tasksMap := make(map[string]cron.EntryID)
	timersMap := make(map[string]context.CancelFunc)
	return &SchedulerService{
		logger:        logger,
		schedulerRepo: schedulerRepo,
		slack:         slack,
		cron:          cronObj,
		tasks:         tasksMap,
		timers:        timersMap,
	}
}

func (s *SchedulerService) GetOne(ctx context.Context, taskID string) (*smodels.TaskModel, error) {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errors.E(errors.Invalid, "task not found with given id")
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

	res := make([]string, 0, len(tasks))
	for _, task := range tasks {
		res = append(res, task.ID)
	}

	activeTasks := &smodels.ActiveTasks{ActiveTasks: res}
	return activeTasks, nil
}

func (s *SchedulerService) Insert(ctx context.Context, taskQP smodels.TaskQP) (string, error) {
	taskID := uuid.New().String()
	curTime := helpers.GetCurrentDateTime()
	task := taskQP.ToTaskModel(taskID, curTime)
	err := s.schedulerRepo.Insert(ctx, task)
	if err != nil {
		return "", fmt.Errorf("failed to insert task: %w", err)
	}
	s.ScheduleTask(task)
	return task.ID, nil
}

func (s *SchedulerService) Delete(ctx context.Context, taskID string) error {
	err := s.schedulerRepo.Delete(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.Invalid, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	s.DiscardTaskNow(taskID)
	return nil
}

func (s *SchedulerService) Toggle(ctx context.Context, taskID string) error {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.Invalid, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to fetch task data: %w", err)
	}

	enable := !task.Enable
	err = s.schedulerRepo.UpdateEnable(ctx, taskID, enable)
	if err != nil {
		return fmt.Errorf("failed to update enabale status: %w", err)
	}

	alreadyExecuted := task.Status.IsAlreadyExecuted()
	if alreadyExecuted && !task.IsRecurEnabled {
		if enable {
			s.logger.Info(fmt.Sprintf("Task %s[NR] Is Already Executed Successfully", taskID))
		}
		return nil
	}

	curUnix := helpers.CurrentUTCUnix()
	if curUnix > helpers.Unix(task.EndUnix) {
		s.logger.Info(fmt.Sprintf("Task %s Is Already Expired", taskID))
		return nil
	}

	if enable {
		s.ScheduleTask(task)
	} else {
		s.DiscardTaskNow(taskID)
	}
	return nil
}

func (s *SchedulerService) ExecuteNow(ctx context.Context, taskID string) error {
	task, err := s.schedulerRepo.GetOne(ctx, taskID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return errors.E(errors.Invalid, "task not found with given id")
	}
	if err != nil {
		return fmt.Errorf("failed to fetch task data: %w", err)
	}

	alreadyExecuted := task.Status.IsAlreadyExecuted()
	if alreadyExecuted && !task.IsRecurEnabled {
		s.logger.Info(fmt.Sprintf("Task %s[NR] Is Already Executed Successfully", taskID))
		return nil
	}

	curUnix := helpers.CurrentUTCUnix()
	if curUnix > helpers.Unix(task.EndUnix) {
		s.logger.Info(fmt.Sprintf("Task %s Is Already Expired", taskID))
		return nil
	}

	s.ExecuteTaskNow(task)
	return nil
}
