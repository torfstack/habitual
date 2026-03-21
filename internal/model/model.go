package model

import "time"

type Habit struct {
	ID          int
	Name        string
	Description string
	Target      int    // completions required per period
	Period      string // "day", "week", "month"
	CreatedAt   time.Time

	// Computed fields (not stored)
	HasEntry    bool // entry exists on the specific queried date
	Completed   bool // PeriodCount >= Target
	PeriodCount int  // entries in the current period window
	Streak      int  // consecutive completed periods
}

type Entry struct {
	ID        int
	HabitID   int
	Day       time.Time
	CreatedAt time.Time
}
