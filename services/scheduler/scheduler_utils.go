package scheduler

import (
	// Go Internal Packages
	"context"
	"fmt"
	"time"

	// Local Packages
	models "scheduler/models"
	executer "scheduler/services/executer"
	helpers "scheduler/utils/helpers"

	// External Packages
	"go.uber.org/zap"
)

type timerKey struct {
	kind   string
	taskID string
}

// Start schedules all active tasks from the database and starts the cron runner.
func (s *SchedulerService) Start(ctx context.Context) error {
	curUnix := helpers.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return fmt.Errorf("unable to fetch tasks: %w", err)
	}

	for _, t := range tasks {
		s.scheduleTask(t)
	}

	s.cron.Start()
	s.logger.Info("Successfully Scheduled All Tasks", zap.Int("count", len(tasks)))
	return nil
}

// scheduleTask routes the task to the correct scheduling path based on start time.
func (s *SchedulerService) scheduleTask(t models.Task) {
	curUnix := helpers.CurrentUTCUnix()
	startUnix := helpers.Unix(t.StartUnix)

	switch {
	case curUnix == startUnix:
		s.scheduleTaskNow(t)
	case curUnix < startUnix:
		// Register cancel before the goroutine starts to eliminate the race window
		// where DiscardTaskNow could run before the goroutine registers itself.
		ctx, cancel := context.WithCancel(context.Background())
		key := timerKey{kind: "schedule", taskID: t.ID}
		s.timersMu.Lock()
		s.timers[key] = cancel
		s.timersMu.Unlock()
		go s.scheduleTaskWithDelay(ctx, startUnix.DurationFrom(curUnix), t)
	default:
		s.scheduleExistingTask(t)
	}
}

// scheduleTaskNow adds the task to cron and fires it immediately.
// Cron/v3 does not support immediate first-fire, so the first execution is triggered manually.
// tasksMu is held across the duplicate-check, AddJob, and map-write to prevent concurrent
// calls from double-scheduling the same task and leaking cron entries.
func (s *SchedulerService) scheduleTaskNow(t models.Task) {
	executor := executer.NewExecutorService(s.execCtx, s.logger, t, s.schedulerRepo, s.slack, s.client)

	s.tasksMu.Lock()
	if _, exists := s.tasks[t.ID]; exists {
		s.tasksMu.Unlock()
		s.logger.Error("Task Is Already Scheduled", zap.String("taskId", t.ID))
		return
	}

	go executor.Run()

	if !t.IsRecurEnabled {
		s.tasksMu.Unlock()
		s.logger.Info("Non Recurring Task, Not Added To Cron", zap.String("taskId", t.ID))
		return
	}

	entryID, err := s.cron.AddJob(fmt.Sprintf("@every %ds", t.Recur), executor)
	if err != nil {
		s.tasksMu.Unlock()
		s.logger.Error("Unable To Schedule Task", zap.String("taskId", t.ID), zap.Error(err))
		return
	}
	s.tasks[t.ID] = entryID
	s.tasksMu.Unlock()

	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()
	deletesIn := endUnix.DurationFrom(curUnix) + time.Second

	ctx, cancel := context.WithCancel(context.Background())
	key := timerKey{kind: "discard", taskID: t.ID}
	s.timersMu.Lock()
	s.timers[key] = cancel
	s.timersMu.Unlock()
	go s.discardTaskWithDelay(ctx, deletesIn, t.ID)
}

// scheduleTaskWithDelay calls scheduleTaskNow after the given duration.
// The context is pre-registered by the caller before this goroutine starts.
func (s *SchedulerService) scheduleTaskWithDelay(ctx context.Context, duration time.Duration, t models.Task) {
	key := timerKey{kind: "schedule", taskID: t.ID}
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, key)
		s.timersMu.Unlock()
	}()

	timer := time.NewTimer(duration)
	defer timer.Stop()
	s.logger.Info("Starting Task With Delay", zap.String("taskId", t.ID), zap.Duration("delay", duration))

	select {
	case <-timer.C:
		s.scheduleTaskNow(t)
	case <-ctx.Done():
		s.logger.Info("Cancelled Pending Schedule Timer", zap.String("taskId", t.ID))
	}
}

// scheduleExistingTask handles tasks whose start time has already passed.
// For non-recurring missed tasks it fires immediately; for recurring tasks it
// calculates the next interval and defers.
func (s *SchedulerService) scheduleExistingTask(t models.Task) {
	if !t.IsRecurEnabled && t.Status.IsAlreadyExecuted() {
		s.logger.Info("Non Recurring Task Already Executed, Skipping", zap.String("taskId", t.ID))
		return
	}
	if !t.IsRecurEnabled && t.Recur == 0 {
		s.logger.Info("Executing Non Recurring Missed Task", zap.String("taskId", t.ID))
		s.scheduleTaskNow(t)
		return
	}

	startUnix := helpers.Unix(t.StartUnix)
	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()

	intervalInSeconds := int64(t.Recur)
	if intervalInSeconds == 0 {
		s.logger.Error("Recurring task has zero recur interval, skipping", zap.String("taskId", t.ID))
		return
	}
	nextTriggerIn := time.Duration(intervalInSeconds-(int64(curUnix-startUnix)%intervalInSeconds)) * time.Second
	endDurationIn := endUnix.DurationFrom(curUnix)
	if nextTriggerIn > endDurationIn {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	key := timerKey{kind: "schedule", taskID: t.ID}
	s.timersMu.Lock()
	s.timers[key] = cancel
	s.timersMu.Unlock()
	go s.scheduleTaskWithDelay(ctx, nextTriggerIn, t)
}

// discardTaskNow removes a task from the scheduler and cancels any pending timers.
func (s *SchedulerService) discardTaskNow(taskID string) {
	s.timersMu.Lock()
	scheduleKey := timerKey{kind: "schedule", taskID: taskID}
	if cancelFunc, exists := s.timers[scheduleKey]; exists {
		cancelFunc()
		delete(s.timers, scheduleKey)
	}
	discardKey := timerKey{kind: "discard", taskID: taskID}
	if cancelFunc, exists := s.timers[discardKey]; exists {
		cancelFunc()
		delete(s.timers, discardKey)
	}
	s.timersMu.Unlock()

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
// The context is pre-registered by the caller before this goroutine starts.
func (s *SchedulerService) discardTaskWithDelay(ctx context.Context, duration time.Duration, taskID string) {
	key := timerKey{kind: "discard", taskID: taskID}
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, key)
		s.timersMu.Unlock()
	}()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		s.discardTaskNow(taskID)
	case <-ctx.Done():
		s.logger.Info("Cancelled Pending Discard Timer", zap.String("taskId", taskID))
	}
}

// executeTaskNow executes the task immediately, regardless of its cron schedule.
func (s *SchedulerService) executeTaskNow(t models.Task) {
	s.logger.Info("Executing Task Now", zap.String("taskId", t.ID))
	executor := executer.NewExecutorService(s.execCtx, s.logger, t, s.schedulerRepo, s.slack, s.client)
	go executor.Run()
}
