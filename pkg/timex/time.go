package timex

import (
	"fmt"
	"time"
)

func GetCurrentDateTime() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
}

func GetExpiryTime() string {
	return time.Now().UTC().AddDate(10, 0, 0).Format("2006-01-02T15:04:05.999Z")
}

type Unix int64

func CurrentUTCUnix() Unix {
	return Unix(time.Now().UTC().Unix())
}

func ToUnixFromISTDateTime(scheduleTime, scheduleDate string) (int64, error) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return 0, fmt.Errorf("timex: load IST location: %w", err)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", scheduleDate+" "+scheduleTime, loc)
	if err != nil {
		return 0, fmt.Errorf("timex: parse IST datetime %q %q: %w", scheduleDate, scheduleTime, err)
	}
	return t.Unix(), nil
}

func ToUnixFromUTCTime(utcTime string) (int64, error) {
	t, err := time.Parse("2006-01-02T15:04:05.999Z", utcTime)
	if err != nil {
		return 0, fmt.Errorf("timex: parse UTC time %q: %w", utcTime, err)
	}
	return t.Unix(), nil
}

// DurationFrom returns the duration (a - b) as seconds. Use when a > b (e.g. endUnix.DurationFrom(curUnix)).
func (a Unix) DurationFrom(b Unix) time.Duration {
	return time.Duration(a-b) * time.Second
}
