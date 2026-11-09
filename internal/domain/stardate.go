package domain

import (
	"fmt"
	"time"
)

// Stardate calculates the real-world standard Trek stardate for a given time.Time.
// Format matches the 1000-units-per-year scale (e.g., 2026 -> 103xxx.x).
func Stardate(t time.Time) float64 {
	year := t.Year()

	// Base year 1900 gives:
	// 2024 -> ~101000
	// 2025 -> ~102000
	// 2026 -> ~103000
	base := float64(year-1900) * 1000.0

	// Determine total days in current year (leap year aware)
	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, t.Location())
	endOfYear := time.Date(year+1, time.January, 1, 0, 0, 0, 0, t.Location())
	totalYearDuration := endOfYear.Sub(startOfYear).Seconds()

	// Elapsed duration from Jan 1 00:00:00 to now
	elapsedDuration := t.Sub(startOfYear).Seconds()

	fraction := elapsedDuration / totalYearDuration
	return base + (fraction * 1000.0)
}

// StardateString formats the calculated stardate to 1 decimal place (e.g. "103854.2").
func StardateString(t time.Time) string {
	return fmt.Sprintf("%.1f", Stardate(t))
}
