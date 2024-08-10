package utils

import "time"

func IsTimeWithinRange(requestedTime, startTime, endTime string) bool {
	requested, _ := time.Parse("15:04", requestedTime)
	start, _ := time.Parse("15:04", startTime)
	end, _ := time.Parse("15:04", endTime)

	return requested.Equal(start) || (requested.After(start) && requested.Before(end))
}

func CalculateEndSessionTime(startTime string) string {
	start, _ := time.Parse("15:04", startTime)
	end := start.Add(30 * time.Minute)
	return end.Format("15:04")
}
