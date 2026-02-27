package model

import (
	"math"
	"time"
)

// YearFrac calculates the fractional year between two dates using the
// US 30/360 day-count convention (Excel YEARFRAC basis 0).
//
// Rules:
//   - If d1 is the last day of a month, treat it as day 30.
//   - If d2 is the last day of a month AND d1 was treated as day 30, treat d2 as day 30.
//   - If d1 == 30 and d2 == 31, clamp d2 to 30.
//
// Formula: (360*(y2-y1) + 30*(m2-m1) + (d2-d1)) / 360
func YearFrac(start, end time.Time) float64 {
	y1, m1, d1 := start.Year(), int(start.Month()), start.Day()
	y2, m2, d2 := end.Year(), int(end.Month()), end.Day()

	if d1 == daysInMonth(y1, m1) {
		d1 = 30
	}
	if d2 == daysInMonth(y2, m2) && d1 == 30 {
		d2 = 30
	}
	if d1 == 30 && d2 == 31 {
		d2 = 30
	}

	days := float64(360*(y2-y1) + 30*(m2-m1) + (d2 - d1))
	return days / 360.0
}

// daysInMonth returns the number of days in the given month/year.
func daysInMonth(year, month int) int {
	// Day 0 of next month = last day of this month
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// addFractionalYears adds a fractional number of years to a date.
// For whole years: DATE(YEAR(d)+N, MONTH(d), DAY(d)) — exactly as Excel does.
// For the fractional part: adds proportional days (using 365.25 average).
func addFractionalYears(d time.Time, years float64) time.Time {
	wholeYears := int(years)
	fraction := years - float64(wholeYears)

	result := time.Date(d.Year()+wholeYears, d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if fraction > 0 {
		extraDays := int(math.Round(fraction * 365.25))
		result = result.AddDate(0, 0, extraDays)
	}
	return result
}
