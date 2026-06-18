package models

import (
	// Go Internal Packages
	"fmt"
	"strings"
	"time"

	// Local Packages
	errors "scheduler/errors"
	helpers "scheduler/utils/helpers"
	httpclient "scheduler/utils/httpclient"
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
		t.ExpiresAt = helpers.GetExpiryTime()
	}
}

func (t *CreateRequest) Validate() error {
	ve := errors.ValidationErrs()

	helpers.ValidateDate(ve, "scheduleDate", t.ScheduleDate)
	helpers.ValidateTime(ve, "scheduleTime", t.ScheduleTime)

	if t.Recur < 0 {
		ve.Add("recur", "cannot be negative")
	}
	if !t.IsRecurEnabled && t.Recur != 0 {
		ve.Add("recur", "needs to be 0 for non-recurring task")
	}
	if t.IsRecurEnabled && t.Recur < 3600 {
		ve.Add("recur", "needs to be greater than 1hr if recur is enabled")
	}
	if t.ExpiresAt != "" {
		if _, err := time.Parse("2006-01-02T15:04:05.999Z", t.ExpiresAt); err != nil {
			ve.Add("expiresAt", "Invalid format, expected RFC3339 NANO")
		}
	}
	helpers.ValidateRequiredString(ve, "taskData.taskType", t.TaskData.TaskType)
	if t.TaskData.RequestType == "" {
		ve.Add("taskData.requestType", "cannot be empty")
	} else if err := t.TaskData.RequestType.Validate(); err != nil {
		ve.Add("taskData.requestType", err.Error())
	}
	helpers.ValidateRequiredString(ve, "taskData.url", t.TaskData.URL)
	if t.Status.LastExecutedAt != "" || t.Status.ExceptionMessage != "" {
		ve.Add("status", "need to be empty for new task")
	}

	if ve.Len() == 0 {
		startUnix, err := helpers.ToUnixFromISTDateTime(t.ScheduleTime, t.ScheduleDate)
		if err != nil {
			ve.Add("scheduleDate and Time", "failed to parse: "+err.Error())
		} else {
			endUnix, err := helpers.ToUnixFromUTCTime(t.ExpiresAt)
			if err != nil {
				ve.Add("expiresAt", "failed to parse: "+err.Error())
			} else {
				if helpers.Unix(startUnix) < helpers.CurrentUTCUnix() {
					ve.Add("scheduleDate and Time", "must be greater than current time")
				}
				if helpers.Unix(endUnix) < helpers.CurrentUTCUnix() || startUnix > endUnix {
					ve.Add("expiresAt", "must be greater than current & schedule time")
				}
			}
		}
	}

	return ve.Err()
}

func (t *CreateRequest) ToTask(taskID, curTime string) (Task, error) {
	startUnix, err := helpers.ToUnixFromISTDateTime(t.ScheduleTime, t.ScheduleDate)
	if err != nil {
		return Task{}, fmt.Errorf("toTask: %w", err)
	}
	endUnix, err := helpers.ToUnixFromUTCTime(t.ExpiresAt)
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
