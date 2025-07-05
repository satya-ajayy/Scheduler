package helpers

import "time"

func GetCurrentDateTime() string {
	utcTime := time.Now().UTC()
	return utcTime.Format("2006-01-02T15:04:05.999Z")
}

func GetPrevTime() string {
	oneYearAgo := time.Now().UTC().AddDate(-1, 0, 0)
	return oneYearAgo.Format("2006-01-02T15:04:05.999Z")
}

func GetExpiryTime() string {
	tenYearsFromNow := time.Now().UTC().AddDate(10, 0, 0)
	return tenYearsFromNow.Format("2006-01-02T15:04:05.999Z")
}

type Unix int64

// CurrentUTCUnix returns the current UTC unix time.
func CurrentUTCUnix() Unix {
	return Unix(time.Now().UTC().Unix())
}

func ToUnixFromISTDateTime(scheduleTime, scheduleDate string) int64 {
	year, month, day := ParseDate(scheduleDate)
	hours, minutes := ParseTime(scheduleTime)
	loc, _ := time.LoadLocation("Asia/Kolkata")
	t := time.Date(year, month, day, hours, minutes, 0, 0, loc)
	return t.Unix()
}

func ToUnixFromUTCTime(utcTime string) int64 {
	t, _ := time.Parse("2006-01-02T15:04:05.999Z", utcTime)
	return t.Unix()
}

// Sub returns the difference between a and b in time duration
func (a Unix) Sub(b Unix, reverse bool) time.Duration {
	if reverse {
		return time.Duration(b-a) * time.Second
	}
	return time.Duration(a-b) * time.Second
}

// ParseDate is used to parse YYYY-MM-DD
func ParseDate(dateStr string) (int, time.Month, int) {
	t, _ := time.Parse("2006-01-02", dateStr)
	return t.Year(), t.Month(), t.Day()
}

// ParseTime is used to parse HH:MM
func ParseTime(timeStr string) (int, int) {
	t, _ := time.Parse("15:04", timeStr)
	return t.Hour(), t.Minute()
}
