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
)

// Start starts the scheduler.
// It schedules all the active tasks that are in the database.
func (s *SchedulerService) Start(ctx context.Context) error {
	curUnix := helpers.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return fmt.Errorf("Unable To Fetch Tasks Due To %v", err)
	}

	for _, t := range tasks {
		s.ScheduleTask(t)
	}

	s.cron.Start()
	s.logger.Info(fmt.Sprintf("Successfully Scheduled All Tasks (%d)", len(tasks)))
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

// ScheduleTaskNow adds the task to the cron.
// Runs the task immediately because cron/v3 doesn't support immediate scheduling.
// And also triggers a goroutine to discard the task after the end time.
// It returns an error if the task is already scheduled.
func (s *SchedulerService) ScheduleTaskNow(t smodels.TaskModel) {
	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if _, exists := s.tasks[t.ID]; exists {
		s.logger.Error(fmt.Sprintf("Task: %s Is Already Scheduled", t.ID))
	}

	executor := executer.NewExecutorService(s.logger, t, s.schedulerRepo, s.slack)
	updatedInterval := fmt.Sprintf("@every %ds", t.Recur)

	// Runs the task in a separate goroutine, this shouldn't be blocking
	go executor.Run()
	if t.IsRecurEnabled == false {
		s.logger.Info("Non Recurring Task, So Not Added To Cron")
		return
	}

	entryID, err := s.cron.AddJob(updatedInterval, executor)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Unable To Schedule Task: %s due to %v", t.ID, err))
		return
	}
	s.tasks[t.ID] = entryID
	deleteBuffer := time.Second
	deletesIn := endUnix.Sub(curUnix, false) + deleteBuffer
	go s.discardTaskWithDelay(deletesIn, t.ID)
	return
}

// scheduleTaskWithDelay schedules the task after the duration.
// calls ScheduleTaskNow after the duration.
// uses context cancellation to allow cancelling the scheduled task before it runs.
func (s *SchedulerService) scheduleTaskWithDelay(duration time.Duration, t smodels.TaskModel) {
	ctx, cancel := context.WithCancel(context.Background())
	s.timersMu.Lock()
	scheduleKey := "schedule_" + t.ID
	s.timers[scheduleKey] = cancel
	s.timersMu.Unlock()

	ticker := time.NewTicker(duration)
	s.logger.Info(fmt.Sprintf("Starting task %s with delay of %s", t.ID, duration))

	defer ticker.Stop()
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, scheduleKey)
		s.timersMu.Unlock()
	}()

	for {
		select {
		case <-ticker.C:
			s.ScheduleTaskNow(t)
			return
		case <-ctx.Done():
			s.logger.Info(fmt.Sprintf("Cancelled Pending Schedule Timer: %s", t.ID))
			return
		}
	}
}

// scheduleExistingTask schedules the existing task.
// it calculates the next recurred time and then adds to the cron.
// beware: panics if the task.StartUnix is greater than the current time.
func (s *SchedulerService) scheduleExistingTask(t smodels.TaskModel) {
	if !t.IsRecurEnabled && t.Recur == 0 {
		s.logger.Info(fmt.Sprintf("Executing Non Recurring Missed Task %s", t.ID))
		s.ScheduleTaskNow(t)
		return
	}

	startUnix := helpers.Unix(t.StartUnix)
	endUnix := helpers.Unix(t.EndUnix)
	curUnix := helpers.CurrentUTCUnix()

	// parsing the interval
	intervalInSeconds := int64(t.Recur)
	nextTriggerIn := time.Duration(intervalInSeconds-(int64(curUnix-startUnix)%intervalInSeconds)) * time.Second
	endDurationIn := endUnix.Sub(curUnix, false)
	if nextTriggerIn > endDurationIn {
		return
	}
	go s.scheduleTaskWithDelay(nextTriggerIn, t)
}

// DiscardTaskNow removes a task from the scheduler
// also cancels any pending timers for this task ID.
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
		s.logger.Info(fmt.Sprintf("Successfully Discarded Task: %s", taskID))
		return
	}
	s.logger.Info(fmt.Sprintf("No Active Task %s Found To Discard", taskID))
}

// discardTaskWithDelay discards the task after the duration.
// Uses context cancellation to allow cancelling the discard operation before it runs.
func (s *SchedulerService) discardTaskWithDelay(duration time.Duration, taskID string) {
	ctx, cancel := context.WithCancel(context.Background())
	s.timersMu.Lock()
	discardKey := "discard_" + taskID
	s.timers[discardKey] = cancel
	s.timersMu.Unlock()

	ticker := time.NewTicker(duration)

	defer ticker.Stop()
	defer func() {
		s.timersMu.Lock()
		delete(s.timers, discardKey)
		s.timersMu.Unlock()
	}()

	for {
		select {
		case <-ticker.C:
			s.DiscardTaskNow(taskID)
			return
		case <-ctx.Done():
			s.logger.Info(fmt.Sprintf("Cancelled Pending Discard Timer: %s", taskID))
			return
		}
	}
}

// ExecuteTaskNow executes the task immediately, regardless of its cron schedule.
func (s *SchedulerService) ExecuteTaskNow(task smodels.TaskModel) {
	s.logger.Info(fmt.Sprintf("Executing Task %s Now", task.ID))
	executor := executer.NewExecutorService(s.logger, task, s.schedulerRepo, s.slack)
	go executor.Run()
}
