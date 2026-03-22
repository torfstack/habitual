package components

import (
	"fmt"
	"time"

	"habitual/internal/model"
)

// --- Calendar helpers ---

// firstOfMonth returns the first day of the month containing t.
func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// prevMonth returns the first day of the previous month.
func prevMonth(month time.Time) time.Time {
	return firstOfMonth(month.AddDate(0, -1, 0))
}

// nextMonth returns the first day of the next month.
func nextMonth(month time.Time) time.Time {
	return firstOfMonth(month.AddDate(0, 1, 0))
}

// isCurrentMonth reports whether month is the same year+month as today.
func isCurrentMonth(month time.Time) bool {
	now := time.Now()
	return month.Year() == now.Year() && month.Month() == now.Month()
}

// isSameMonth reports whether d and month share the same year+month.
func isSameMonth(d, month time.Time) bool {
	return d.Year() == month.Year() && d.Month() == month.Month()
}

// isSameDay reports whether a and b fall on the same calendar day.
func isSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// isFutureDay reports whether d is strictly after today.
func isFutureDay(d time.Time) bool {
	today := time.Now().Truncate(24 * time.Hour)
	return d.After(today)
}

// calendarDays returns all days to display in a calendar grid for the given month.
// The grid always starts on Monday and ends on Sunday, padding with days from
// adjacent months to complete the first and last weeks.
func calendarDays(month time.Time) []time.Time {
	first := firstOfMonth(month)

	// ISO weekday: Mon=1 … Sun=7
	wd := int(first.Weekday())
	if wd == 0 {
		wd = 7
	}
	start := first.AddDate(0, 0, -(wd - 1))

	last := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	wd = int(last.Weekday())
	if wd == 0 {
		wd = 7
	}
	end := last.AddDate(0, 0, 7-wd)

	var days []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}


func formatDateLabel(t time.Time) string {
	today := time.Now().Truncate(24 * time.Hour)
	d := t.Truncate(24 * time.Hour)
	switch {
	case d.Equal(today):
		return "Today"
	case d.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return d.Format("Mon, Jan 2")
	}
}

func isToday(t time.Time) bool {
	today := time.Now().Truncate(24 * time.Hour)
	return t.Truncate(24 * time.Hour).Equal(today)
}

func dateParam(t time.Time) string {
	return t.Format("2006-01-02")
}

func allCompleted(habits []model.Habit) bool {
	for _, h := range habits {
		if !h.Completed {
			return false
		}
	}
	return true
}

func freqLabel(target int, period string) string {
	if target == 1 {
		switch period {
		case "week":
			return "1× / wk"
		case "month":
			return "1× / mo"
		}
		return ""
	}
	switch period {
	case "week":
		return fmt.Sprintf("%d× / wk", target)
	case "month":
		return fmt.Sprintf("%d× / mo", target)
	}
	return ""
}
