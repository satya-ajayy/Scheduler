package task

import (
	"fmt"
	"strings"
	"time"

	errors "scheduler/internal/errors"
	httpclient "scheduler/pkg/httpclient"
	timex "scheduler/pkg/timex"
)

type Data struct {
	TaskType    string            `json:"taskType" bson:"taskType"`
	RequestType httpclient.Method `json:"requestType" bson:"requestType"`
	URL         string            `json:"url" bson:"url"`
	QueryParams map[string]any    `json:"queryParams" bson:"queryParams"`
	Headers     map[string]string `json:"headers" bson:"headers"`
	RequestBody map[string]any    `json:"requestBody" bson:"requestBody"`
}

type Status struct {
	LastExecutedAt   string `json:"lastExecutedAt" bson:"lastExecutedAt"` // UTC
	IsComplete       bool   `json:"isComplete" bson:"isComplete"`
	ExceptionMessage string `json:"exceptionMessage" bson:"exceptionMessage"`
}

type Task struct {
	ID               string `json:"_id" bson:"_id"`
	Schedule         string `json:"schedule" bson:"schedule"`
	Enable           bool   `json:"enable" bson:"enable"`
	ScheduleDate     string `json:"scheduleDate" bson:"scheduleDate"` // IST
	ScheduleTime     string `json:"scheduleTime" bson:"scheduleTime"` // IST
	Recur            int    `json:"recur" bson:"recur"`
	IsRecurEnabled   bool   `json:"isRecurEnabled" bson:"isRecurEnabled"`
	NumberOfAttempts int    `json:"numberOfAttempts" bson:"numberOfAttempts"`
	CreatedAt        string `json:"createdAt" bson:"createdAt"` // UTC
	UpdatedAt        string `json:"updatedAt" bson:"updatedAt"` // UTC
	ExpiresAt        string `json:"expiresAt" bson:"expiresAt"` // UTC
	StartUnix        int64  `json:"startUnix" bson:"startUnix"` // UTC
	EndUnix          int64  `json:"endUnix" bson:"endUnix"`     // UTC
	TaskData         Data   `json:"taskData" bson:"taskData"`
	Status           Status `json:"status" bson:"status"`
}

type CreateRequest struct {
	Schedule         string `json:"schedule"`
	Enable           bool   `json:"enable"`
	ScheduleDate     string `json:"scheduleDate"` // IST
	ScheduleTime     string `json:"scheduleTime"` // IST
	Recur            int    `json:"recur"`
	IsRecurEnabled   bool   `json:"isRecurEnabled"`
	NumberOfAttempts int    `json:"numberOfAttempts"`
	ExpiresAt        string `json:"expiresAt"` // UTC
	TaskData         Data   `json:"taskData"`
	Status           Status `json:"status"`
}

type ActiveList struct {
	ActiveTasks []string `json:"activeTasks"`
}

func (s *Status) IsAlreadyExecuted() bool {
	return s.LastExecutedAt != ""
}

// Normalize sets defaults and normalizes fields. Call before Validate.
func (t *CreateRequest) Normalize() {
	t.Schedule = strings.ToUpper(t.Schedule)
	if t.Schedule == "" {
		t.Schedule = "NOW"
	}
	if t.NumberOfAttempts == 0 {
		t.NumberOfAttempts = 3
	}
	if t.ExpiresAt == "" {
		t.ExpiresAt = timex.GetExpiryTime()
	}
}

func (t *CreateRequest) Validate() error {
	ve := errors.ValidationErrs()

	if t.ScheduleDate == "" {
		ve.Add("scheduleDate", "cannot be empty")
	}
	if t.ScheduleTime == "" {
		ve.Add("scheduleTime", "cannot be empty")
	}
	if t.Recur < 0 {
		ve.Add("recur", "cannot be negative")
	}
	if !t.IsRecurEnabled && t.Recur != 0 {
		ve.Add("recur", "needs to be 0 for non-recurring task")
	}
	if t.IsRecurEnabled && t.Recur < 3600 {
		ve.Add("recur", "needs to be greater than 1hr if recur is enabled")
	}
	if t.ScheduleDate != "" {
		if _, err := time.Parse("2006-01-02", t.ScheduleDate); err != nil {
			ve.Add("scheduleDate", "Invalid format, expected YYYY-MM-DD")
		}
	}
	if t.ScheduleTime != "" {
		if _, err := time.Parse("15:04", t.ScheduleTime); err != nil {
			ve.Add("scheduleTime", "Invalid format, expected HH:MM (IST)")
		}
	}
	if t.ExpiresAt != "" {
		if _, err := time.Parse("2006-01-02T15:04:05.999Z", t.ExpiresAt); err != nil {
			ve.Add("expiresAt", "Invalid format, expected RFC3339 NANO")
		}
	}
	if t.TaskData.TaskType == "" {
		ve.Add("taskData.taskType", "cannot be empty")
	}
	if t.TaskData.RequestType == "" {
		ve.Add("taskData.requestType", "cannot be empty")
	}
	if t.TaskData.RequestType != "" {
		if err := t.TaskData.RequestType.Validate(); err != nil {
			ve.Add("taskData.requestType", err.Error())
		}
	}
	if t.TaskData.URL == "" {
		ve.Add("taskData.url", "cannot be empty")
	}
	if t.Status.LastExecutedAt != "" || t.Status.ExceptionMessage != "" {
		ve.Add("status", "need to be empty for new task")
	}

	if ve.Len() == 0 {
		startUnix, err := timex.ToUnixFromISTDateTime(t.ScheduleTime, t.ScheduleDate)
		if err != nil {
			ve.Add("scheduleDate and Time", "failed to parse: "+err.Error())
		} else {
			endUnix, err := timex.ToUnixFromUTCTime(t.ExpiresAt)
			if err != nil {
				ve.Add("expiresAt", "failed to parse: "+err.Error())
			} else {
				if timex.Unix(startUnix) < timex.CurrentUTCUnix() {
					ve.Add("scheduleDate and Time", "must be greater than current time")
				}
				if timex.Unix(endUnix) < timex.CurrentUTCUnix() || startUnix > endUnix {
					ve.Add("expiresAt", "must be greater than current & schedule time")
				}
			}
		}
	}

	return ve.Err()
}

func (t *CreateRequest) ToTask(taskID, curTime string) (Task, error) {
	startUnix, err := timex.ToUnixFromISTDateTime(t.ScheduleTime, t.ScheduleDate)
	if err != nil {
		return Task{}, fmt.Errorf("toTask: %w", err)
	}
	endUnix, err := timex.ToUnixFromUTCTime(t.ExpiresAt)
	if err != nil {
		return Task{}, fmt.Errorf("toTask: %w", err)
	}
	return Task{
		ID:               taskID,
		Schedule:         t.Schedule,
		Enable:           t.Enable,
		ScheduleDate:     t.ScheduleDate,
		ScheduleTime:     t.ScheduleTime,
		Recur:            t.Recur,
		IsRecurEnabled:   t.IsRecurEnabled,
		NumberOfAttempts: t.NumberOfAttempts,
		CreatedAt:        curTime,
		UpdatedAt:        curTime,
		ExpiresAt:        t.ExpiresAt,
		StartUnix:        startUnix,
		EndUnix:          endUnix,
		TaskData:         t.TaskData,
		Status:           t.Status,
	}, nil
}
