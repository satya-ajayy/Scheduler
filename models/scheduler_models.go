package models

import (
	// 	Go Internal Packages
	"strings"
	"time"

	//Local Packages
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

func (s *TaskModel) ValidateCreation() error {
	ve := errors.ValidationErrs()

	s.Schedule = strings.ToUpper(s.Schedule)
	if s.Schedule == "" {
		s.Schedule = "NOW"
	}
	if s.ScheduleDate == "" {
		ve.Add("scheduleDate", "cannot be empty")
	}
	if s.ScheduleTime == "" {
		ve.Add("scheduleTime", "cannot be empty")
	}
	if _, err := time.Parse("2006-01-02", s.ScheduleDate); err != nil {
		ve.Add("scheduleDate", "Invalid format, expected YYYY-MM-DD")
	}
	if _, err := time.Parse("15:04", s.ScheduleTime); err != nil {
		ve.Add("scheduleTime", "Invalid format, expected HH:MM (IST)")
	}
	if s.NumberOfAttempts == 0 {
		s.NumberOfAttempts = 3
	}
	if s.ExpiresAt != "" {
		if _, err := time.Parse("2006-01-02T15:04:05.999Z", s.ExpiresAt); err != nil {
			ve.Add("expiresAt", "Invalid format, expected RFC3339 NANO")
		}
	}
	if s.ExpiresAt == "" {
		s.ExpiresAt = utils.GetExpiryTime()
	}
	if s.TaskData.TaskType == "" {
		ve.Add("taskData.taskType", "cannot be empty")
	}
	if s.TaskData.RequestType == "" {
		ve.Add("taskData.requestType", "cannot be empty")
	}
	err := s.TaskData.RequestType.Validate()
	if err != nil {
		ve.Add("taskData.requestType", err.Error())
	}
	if s.TaskData.URL == "" {
		ve.Add("taskData.url", "cannot be empty")
	}
	if s.Status.LastExecutedAt != "" || s.Status.ExceptionMessage != "" {
		ve.Add("status", "need to be empty for new task")
	}

	if ve.Len() == 0 {
		StartUnix := utils.ToUnixFromISTDateTime(s.ScheduleTime, s.ScheduleDate)
		EndUnix := utils.ToUnixFromUTCTime(s.ExpiresAt)
		if utils.Unix(StartUnix) < utils.CurrentUTCUnix() {
			ve.Add("scheduleDate and Time", "must be greater than current time")
		}
		if utils.Unix(EndUnix) < utils.CurrentUTCUnix() || StartUnix > EndUnix {
			ve.Add("expiresAt", "must be greater than current time")
		}
		s.StartUnix = StartUnix
		s.EndUnix = EndUnix
	}
	return ve.Err()
}
