package scheduler

import (
	// Go Internal Packages
	"context"
	"fmt"
	"time"

	// Local Packages
	smodels "scheduler/models"
	executer "scheduler/services/executer"
	utils "scheduler/utils"
)

// Start starts the scheduler.
// It schedules all the active tasks that read from the database.
func (s *SchedulerService) Start(ctx context.Context) error {
	curUnix := utils.CurrentUTCUnix()
	tasks, err := s.schedulerRepo.GetActive(ctx, curUnix)
	if err != nil {
		return fmt.Errorf("Unable To Fetch Tasks Due To %v", err)
	}

	for _, t := range tasks {
		s.ScheduleTask(t)
		//s.logger.Info(fmt.Sprintf("Successfully Scheduled Task :: %s", t.ID))
	}

	s.cron.Start()
	s.logger.Info(fmt.Sprintf("Successfully Scheduled All Tasks (%d)", len(tasks)))
	return nil
}

// ScheduleTask schedules the task based on the start time.
func (s *SchedulerService) ScheduleTask(t smodels.TaskModel) {
	curUnix := utils.CurrentUTCUnix()
	startUnix := utils.Unix(t.StartUnix)

	if curUnix == startUnix {
		err := s.ScheduleTaskNow(t)
		if err != nil {
			s.logger.Error(err.Error())
		}
	} else if curUnix < startUnix {
		go s.scheduleTaskWithDelay(startUnix.Sub(curUnix, false), t)
	} else {
		s.scheduleExistingTask(t)
	}
}

// ScheduleTaskNow adds the task to the cron.
// Runs the task immediately because cron/v3 doesn't support immediate scheduling.
// and also triggers a goroutine to discard the task after the end time.
// It returns an error if the task is already scheduled.
func (s *SchedulerService) ScheduleTaskNow(t smodels.TaskModel) error {
	endUnix := utils.Unix(t.EndUnix)
	curUnix := utils.CurrentUTCUnix()

	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if _, exists := s.tasks[t.ID]; exists {
		return fmt.Errorf("Task :: %s Is Already Scheduled", t.ID)
	}

	executor := executer.NewExecutorService(s.logger, t, s.schedulerRepo)
	updatedInterval := fmt.Sprintf("@every %ds", t.Recur)
	// runs the task in separate goroutine, this shouldn't be blocking
	go executor.Run()
	if t.IsRecurEnabled == false {
		s.logger.Info("Non Recurring Task, So Not Added To Cron")
		return nil
	}

	entryID, err := s.cron.AddJob(updatedInterval, executor)
	if err != nil {
		return fmt.Errorf("Unable To Schedule Task :: %s due to %v", t.ID, err)
	}
	s.tasks[t.ID] = entryID
	deleteBuffer := time.Second
	deletesIn := endUnix.Sub(curUnix, false) + deleteBuffer
	go s.discardTaskWithDelay(deletesIn, t.ID)
	return nil
}

// scheduleTaskWithDelay schedules the task after the duration.
// calls ScheduleTaskNow after the duration.
func (s *SchedulerService) scheduleTaskWithDelay(duration time.Duration, t smodels.TaskModel) {
	ticker := time.NewTicker(duration)
	s.logger.Info(fmt.Sprintf("Starting task %s with delay at %s", t.ID, duration))

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := s.ScheduleTaskNow(t)
			if err != nil {
				s.logger.Error(fmt.Sprintf("Unable To Schedule Task :: %s due to %v", t.ID, err))
			}
			s.logger.Info(fmt.Sprintf("Successfully Executed Task :: %s", t.ID))
			return
		}
	}
}

// scheduleExistingTask schedules the existing task.
// it calculates the next recur time and then adds to the cron.
// beware: panics if the task.StartUnix is greater than the current time.
func (s *SchedulerService) scheduleExistingTask(t smodels.TaskModel) {
	startUnix := utils.Unix(t.StartUnix)
	endUnix := utils.Unix(t.EndUnix)
	curUnix := utils.CurrentUTCUnix()

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
// and won't stop the task if it is running.
// if the task is not found in scheduler, it logs a message.
func (s *SchedulerService) DiscardTaskNow(taskID string) {
	s.tasksMu.Lock()
	defer s.tasksMu.Unlock()
	if entryID, exists := s.tasks[taskID]; exists {
		s.cron.Remove(entryID)
		delete(s.tasks, taskID)
		s.logger.Info(fmt.Sprintf("Successfully Discarded Task :: %s", taskID))
		return
	}
	s.logger.Info(fmt.Sprintf("No Active Task With ID :: %s Found To Discard", taskID))
}

// discardTaskWithDelay discards the task after the duration.
func (s *SchedulerService) discardTaskWithDelay(duration time.Duration, taskID string) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.DiscardTaskNow(taskID)
			return
		}
	}
}
