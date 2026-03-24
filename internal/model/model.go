package model

import (
	"time"
)

type Habit struct {
	ID          int
	UserID      int
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

type User struct {
	ID          int
	GoogleSub   string
	Email       string
	Name        string
	PictureURL  string
	CreatedAt   time.Time
	LastLoginAt time.Time
}

// DaySummary holds completion status for a single calendar day.
type DaySummary struct {
	HasEntry bool // at least one habit has an entry
	AllDone  bool // all active habits have entries
}
