package scheduler

import (
	// Go Internal Packages
	"context"
	"fmt"
	"time"

	// Local Packages
	smodels "scheduler/models"
	executer "scheduler/services/executer"
	helpers "scheduler/utils/helpers"

	// External Packages
	"go.uber.org/zap"
)

// Start schedules all active tasks from the database and starts the cron runner.
func (s *SchedulerService) Start(ctx context.Context) error {
	curUnix := helpers.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return fmt.Errorf("unable to fetch tasks: %w", err)
	}

	for _, t := range tasks {
		s.ScheduleTask(t)
	}

	s.cron.Start()
	s.logger.Info("Successfully Scheduled All Tasks", zap.Int("count", len(tasks)))
	return nil
}

// ScheduleTask schedules the task based on the start time.
func (s *SchedulerService) ScheduleTask(t smodels.TaskModel) {
	curUnix := helpers.CurrentUTCUnix()
	startUnix := helpers.Unix(t.StartUnix)

	if curUnix == startUnix {
		s.ScheduleTaskNow(t)
	} else if curUnix < startUnix {
		go s.scheduleTaskWithDelay(startUnix.Sub(curUnix, false), t)
	} else {
		s.scheduleExistingTask(t)
	}
}

// ScheduleTaskNow adds the task to the cron and runs it immediately.
// Cron/v3 does not support immediate first-fire, so we trigger the first execution manually.
// Also starts a goroutine to discard the task after end time.
func (s *SchedulerService) ScheduleTaskNow(t smodels.TaskModel) {
	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if _, exists := s.tasks[t.ID]; exists {
		s.logger.Error("Task Is Already Scheduled", zap.String("taskId", t.ID))
	}

	executor := executer.NewExecutorService(s.logger, t, s.schedulerRepo, s.slack)

	// Runs the task in a separate goroutine, this shouldn't be blocking
	go executor.Run()
	if !t.IsRecurEnabled {
		s.logger.Info("Non Recurring Task, Not Added To Cron", zap.String("taskId", t.ID))
		return
	}

	entryID, err := s.cron.AddJob(fmt.Sprintf("@every %ds", t.Recur), executor)
	if err != nil {
		s.logger.Error("Unable To Schedule Task", zap.String("taskId", t.ID), zap.Error(err))
		return
	}
	s.tasks[t.ID] = entryID

	deleteBuffer := time.Second
	deletesIn := endUnix.Sub(curUnix, false) + deleteBuffer
	go s.discardTaskWithDelay(deletesIn, t.ID)
}

// scheduleTaskWithDelay calls ScheduleTaskNow after the given duration.
// Context cancellation allows stopping the scheduled task before it fires.
func (s *SchedulerService) scheduleTaskWithDelay(duration time.Duration, t smodels.TaskModel) {
	ctx, cancel := context.WithCancel(context.Background())
	s.timersMu.Lock()
	scheduleKey := "schedule_" + t.ID
	s.timers[scheduleKey] = cancel
	s.timersMu.Unlock()

	timer := time.NewTimer(duration)
	s.logger.Info("Starting Task With Delay", zap.String("taskId", t.ID), zap.Duration("delay", duration))

	defer timer.Stop()
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, scheduleKey)
		s.timersMu.Unlock()
	}()

	select {
	case <-timer.C:
		s.ScheduleTaskNow(t)
	case <-ctx.Done():
		s.logger.Info("Cancelled Pending Schedule Timer", zap.String("taskId", t.ID))
	}
}

// scheduleExistingTask handles tasks whose start time has already passed.
// For non-recurring missed tasks it fires immediately; for recurring tasks it
// calculates the next interval and defers.
func (s *SchedulerService) scheduleExistingTask(t smodels.TaskModel) {
	if !t.IsRecurEnabled && t.Recur == 0 {
		s.logger.Info("Executing Non Recurring Missed Task", zap.String("taskId", t.ID))
		s.ScheduleTaskNow(t)
		return
	}

	startUnix := helpers.Unix(t.StartUnix)
	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()

	// Parsing the interval to calculate when the next trigger fires
	intervalInSeconds := int64(t.Recur)
	nextTriggerIn := time.Duration(intervalInSeconds-(int64(curUnix-startUnix)%intervalInSeconds)) * time.Second
	endDurationIn := endUnix.Sub(curUnix, false)
	if nextTriggerIn > endDurationIn {
		return
	}
	go s.scheduleTaskWithDelay(nextTriggerIn, t)
}

// DiscardTaskNow removes a task from the scheduler and cancels any pending timers.
func (s *SchedulerService) DiscardTaskNow(taskID string) {
	s.timersMu.Lock()
	// Check for schedule timer
	scheduleKey := "schedule_" + taskID
	if cancelFunc, exists := s.timers[scheduleKey]; exists {
		cancelFunc()
		delete(s.timers, scheduleKey)
	}

	// Check for discard timer
	discardKey := "discard_" + taskID
	if cancelFunc, exists := s.timers[discardKey]; exists {
		cancelFunc()
		delete(s.timers, discardKey)
	}
	s.timersMu.Unlock()

	// Remove from cron scheduler
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if entryID, exists := s.tasks[taskID]; exists {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
		s.logger.Info("Successfully Discarded Task", zap.String("taskId", taskID))
		return
	}
	s.logger.Info("No Active Task Found To Discard", zap.String("taskId", taskID))
}

// discardTaskWithDelay removes the task after the given duration.
// Context cancellation allows stopping the discard before it fires.
func (s *SchedulerService) discardTaskWithDelay(duration time.Duration, taskID string) {
	ctx, cancel := context.WithCancel(context.Background())
	s.timersMu.Lock()
	discardKey := "discard_" + taskID
	s.timers[discardKey] = cancel
	s.timersMu.Unlock()

	timer := time.NewTimer(duration)

	defer timer.Stop()
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, discardKey)
		s.timersMu.Unlock()
	}()

	select {
	case <-timer.C:
		s.DiscardTaskNow(taskID)
	case <-ctx.Done():
		s.logger.Info("Cancelled Pending Discard Timer", zap.String("taskId", taskID))
	}
}

// ExecuteTaskNow executes the task immediately, regardless of its cron schedule.
func (s *SchedulerService) ExecuteTaskNow(task smodels.TaskModel) {
	s.logger.Info("Executing Task Now", zap.String("taskId", task.ID))
	executor := executer.NewExecutorService(s.logger, task, s.schedulerRepo, s.slack)
	go executor.Run()
}
