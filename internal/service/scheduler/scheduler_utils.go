package scheduler

import (
	"context"
	"fmt"
	"time"

	executer "scheduler/internal/service/executer"
	task "scheduler/internal/task"
	timex "scheduler/pkg/timex"

	"go.uber.org/zap"
)

type timerKey struct {
	kind   string
	taskID string
}

// Start schedules all active tasks from the database and starts the cron runner.
func (s *SchedulerService) Start(ctx context.Context) error {
	curUnix := timex.CurrentUTCUnix()
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
func (s *SchedulerService) scheduleTask(t task.Task) {
	curUnix := timex.CurrentUTCUnix()
	startUnix := timex.Unix(t.StartUnix)

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
func (s *SchedulerService) scheduleTaskNow(t task.Task) {
	s.tasksMu.Lock()
	_, exists := s.tasks[t.ID]
	s.tasksMu.Unlock()

	if exists {
		s.logger.Error("Task Is Already Scheduled", zap.String("taskId", t.ID))
		return
	}

	executor := executer.NewExecutorService(s.ctx, s.logger, t, s.schedulerRepo, s.slack, s.client)
	go executor.Run()

	if !t.IsRecurEnabled {
		s.logger.Info("Non Recurring Task, Not Added To Cron", zap.String("taskId", t.ID))
		return
	}

	// cron.AddJob is released outside the lock to prevent holding tasksMu
	// across an external call that may itself block or call back into this service.
	entryID, err := s.cron.AddJob(fmt.Sprintf("@every %ds", t.Recur), executor)
	if err != nil {
		s.logger.Error("Unable To Schedule Task", zap.String("taskId", t.ID), zap.Error(err))
		return
	}

	s.tasksMu.Lock()
	s.tasks[t.ID] = entryID
	s.tasksMu.Unlock()

	endUnix := timex.Unix(t.EndUnix)
	curUnix := timex.CurrentUTCUnix()
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
func (s *SchedulerService) scheduleTaskWithDelay(ctx context.Context, duration time.Duration, t task.Task) {
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
func (s *SchedulerService) scheduleExistingTask(t task.Task) {
	if !t.IsRecurEnabled && t.Status.IsAlreadyExecuted() {
		s.logger.Info("Non Recurring Task Already Executed, Skipping", zap.String("taskId", t.ID))
		return
	}
	if !t.IsRecurEnabled && t.Recur == 0 {
		s.logger.Info("Executing Non Recurring Missed Task", zap.String("taskId", t.ID))
		s.scheduleTaskNow(t)
		return
	}

	startUnix := timex.Unix(t.StartUnix)
	endUnix := timex.Unix(t.EndUnix)
	curUnix := timex.CurrentUTCUnix()

	intervalInSeconds := int64(t.Recur)
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
func (s *SchedulerService) executeTaskNow(t task.Task) {
	s.logger.Info("Executing Task Now", zap.String("taskId", t.ID))
	executor := executer.NewExecutorService(s.ctx, s.logger, t, s.schedulerRepo, s.slack, s.client)
	go executor.Run()
}
