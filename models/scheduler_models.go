package models

import (
	// Go Internal Packages
	"strings"
	"time"

	// Local Packages
	errors "scheduler/errors"
	utils "scheduler/utils"
)

type TaskData struct {
	TaskType    string                 `json:"taskType" bson:"taskType"`
	RequestType utils.HttpRequestType  `json:"requestType" bson:"requestType"`
	URL         string                 `json:"url" bson:"url"`
	QueryParams map[string]interface{} `json:"queryParams" bson:"queryParams"`
	Headers     map[string]string      `json:"headers" bson:"headers"`
	RequestBody map[string]interface{} `json:"requestBody" bson:"requestBody"`
}

type Status struct {
	LastExecutedAt   string `json:"lastExecutedAt" bson:"lastExecutedAt"` // UTC
	IsComplete       bool   `json:"isComplete" bson:"isComplete"`
	ExceptionMessage string `json:"exceptionMessage" bson:"exceptionMessage"`
}
type TaskModel struct {
	ID               string   `json:"_id" bson:"_id"`
	Schedule         string   `json:"schedule" bson:"schedule"`
	Enable           bool     `json:"enable" bson:"enable"`
	ScheduleDate     string   `json:"scheduleDate" bson:"scheduleDate"` // IST
	ScheduleTime     string   `json:"scheduleTime" bson:"scheduleTime"` // IST
	Recur            int      `json:"recur" bson:"recur"`
	IsRecurEnabled   bool     `json:"isRecurEnabled" bson:"isRecurEnabled"`
	NumberOfAttempts int      `json:"numberOfAttempts" bson:"numberOfAttempts"`
	CreatedAt        string   `json:"createdAt" bson:"createdAt"` // UTC
	UpdatedAt        string   `json:"updatedAt" bson:"updatedAt"` // UTC
	ExpiresAt        string   `json:"expiresAt" bson:"expiresAt"` // UTC
	StartUnix        int64    `json:"startUnix" bson:"startUnix"` // UTC
	EndUnix          int64    `json:"endUnix" bson:"endUnix"`     // UTC
	TaskData         TaskData `json:"taskData" bson:"taskData"`
	Status           Status   `json:"status" bson:"status"`
}

type SetEnableParams struct {
	Enable bool `json:"enable" bson:"enable"`
}

func (s *Status) IsAlreadyExecuted() bool {
	return s.LastExecutedAt != ""
}

func (t *TaskModel) ValidateCreation() error {
	ve := errors.ValidationErrs()

	t.Schedule = strings.ToUpper(t.Schedule)
	if t.Schedule == "" {
		t.Schedule = "NOW"
	}
	if t.ScheduleDate == "" {
		ve.Add("scheduleDate", "cannot be empty")
	}
	if t.ScheduleTime == "" {
		ve.Add("scheduleTime", "cannot be empty")
	}
	if t.IsRecurEnabled && t.Recur < 3600 {
		ve.Add("recur", "needs to be greater than 1hr if recur is enabled")
	}
	if _, err := time.Parse("2006-01-02", t.ScheduleDate); err != nil {
		ve.Add("scheduleDate", "Invalid format, expected YYYY-MM-DD")
	}
	if _, err := time.Parse("15:04", t.ScheduleTime); err != nil {
		ve.Add("scheduleTime", "Invalid format, expected HH:MM (IST)")
	}
	if t.NumberOfAttempts == 0 {
		t.NumberOfAttempts = 3
	}
	if t.ExpiresAt != "" {
		if _, err := time.Parse("2006-01-02T15:04:05.999Z", t.ExpiresAt); err != nil {
			ve.Add("expiresAt", "Invalid format, expected RFC3339 NANO")
		}
	}
	if t.ExpiresAt == "" {
		t.ExpiresAt = utils.GetExpiryTime()
	}
	if t.TaskData.TaskType == "" {
		ve.Add("taskData.taskType", "cannot be empty")
	}
	if t.TaskData.RequestType == "" {
		ve.Add("taskData.requestType", "cannot be empty")
	}
	err := t.TaskData.RequestType.Validate()
	if err != nil {
		ve.Add("taskData.requestType", err.Error())
	}
	if t.TaskData.URL == "" {
		ve.Add("taskData.url", "cannot be empty")
	}
	if t.Status.LastExecutedAt != "" || t.Status.ExceptionMessage != "" {
		ve.Add("status", "need to be empty for new task")
	}

	if ve.Len() == 0 {
		StartUnix := utils.ToUnixFromISTDateTime(t.ScheduleTime, t.ScheduleDate)
		EndUnix := utils.ToUnixFromUTCTime(t.ExpiresAt)
		//if utils.Unix(StartUnix) < utils.CurrentUTCUnix() {
		//	ve.Add("scheduleDate and Time", "must be greater than current time")
		//}
		if utils.Unix(EndUnix) < utils.CurrentUTCUnix() || StartUnix > EndUnix {
			ve.Add("expiresAt", "must be greater than current time")
		}
		t.StartUnix = StartUnix
		t.EndUnix = EndUnix
	}
	return ve.Err()
}
