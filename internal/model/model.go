package model

import "time"

type Habit struct {
	ID          int
	Name        string
	Description string
	Points      int
	CreatedAt   time.Time

	// Computed fields (not stored)
	CompletedToday bool
	Streak         int
}

type Entry struct {
	ID        int
	HabitID   int
	Day       time.Time
	CreatedAt time.Time
}
