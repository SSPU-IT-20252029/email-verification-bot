package schoolyear

import "time"

const LocationName = "Europe/Prague"

func LoadLocation() (*time.Location, error) {
	return time.LoadLocation(LocationName)
}

func Start(t time.Time) int {
	if t.Month() >= time.September {
		return t.Year()
	}
	return t.Year() - 1
}
