package schoolyear

import (
	"testing"
	"time"
)

func d(year int, month time.Month, day int, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC)
}

func TestStart(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want int
	}{
		{"mid school year", d(2025, time.October, 15, 12, 0), 2025},
		{"june still old year", d(2026, time.June, 1, 9, 0), 2025},
		{"last minute of august", d(2026, time.August, 31, 23, 59), 2025},
		{"first minute of september", d(2026, time.September, 1, 0, 0), 2026},
		{"new year still same school year", d(2027, time.January, 1, 0, 1), 2026},
		{"deep in school year", d(2026, time.December, 31, 23, 59), 2026},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Start(tt.now); got != tt.want {
				t.Errorf("Start(%s) = %d, want %d", tt.now, got, tt.want)
			}
		})
	}
}
