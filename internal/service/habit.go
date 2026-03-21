package service

import (
	"context"
	"fmt"
	"time"

	"habitual/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HabitService struct {
	db *pgxpool.Pool
}

func NewHabitService(db *pgxpool.Pool) *HabitService {
	return &HabitService{db: db}
}

// periodKey returns a string key grouping a date into its containing period.
func periodKey(t time.Time, period string) string {
	switch period {
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	case "month":
		return t.Format("2006-01")
	default: // "day"
		return t.Format("2006-01-02")
	}
}

// prevPeriodDate returns a date guaranteed to fall in the previous period.
func prevPeriodDate(t time.Time, period string) time.Time {
	switch period {
	case "week":
		return t.AddDate(0, 0, -7)
	case "month":
		return t.AddDate(0, -1, 0)
	default: // "day"
		return t.AddDate(0, 0, -1)
	}
}

// computeStreak counts consecutive completed periods ending at (or before) date.
// The current period is included only if it has met the target; otherwise the
// count starts from the previous period so an in-progress period doesn't reset
// an otherwise intact streak.
func computeStreak(entries []time.Time, period string, target int, date time.Time) int {
	counts := map[string]int{}
	for _, e := range entries {
		counts[periodKey(e, period)]++
	}

	current := date
	if counts[periodKey(current, period)] < target {
		current = prevPeriodDate(current, period)
	}

	streak := 0
	for {
		if counts[periodKey(current, period)] >= target {
			streak++
			current = prevPeriodDate(current, period)
		} else {
			break
		}
	}
	return streak
}

func (s *HabitService) List(ctx context.Context, date time.Time) ([]model.Habit, error) {
	day := date.Format("2006-01-02")

	rows, err := s.db.Query(ctx, `
		SELECT
			h.id, h.name, h.description, h.target, h.period, h.created_at,
			EXISTS(
				SELECT 1 FROM entries WHERE habit_id = h.id AND day = $1::date
			) AS has_entry,
			pc.cnt AS period_count,
			(pc.cnt >= h.target) AS completed
		FROM habits h
		CROSS JOIN LATERAL (
			SELECT COUNT(*)::int AS cnt
			FROM entries
			WHERE habit_id = h.id
			  AND day >= DATE_TRUNC(h.period, $1::timestamptz)::date
			  AND day <  (DATE_TRUNC(h.period, $1::timestamptz)
			              + CASE h.period
			                  WHEN 'day'   THEN '1 day'::interval
			                  WHEN 'week'  THEN '1 week'::interval
			                  WHEN 'month' THEN '1 month'::interval
			                END)::date
		) pc
		WHERE (h.deleted_at IS NULL OR h.deleted_at::date > $1::date)
		ORDER BY h.created_at ASC
	`, day)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		var h model.Habit
		if err := rows.Scan(
			&h.ID, &h.Name, &h.Description, &h.Target, &h.Period, &h.CreatedAt,
			&h.HasEntry, &h.PeriodCount, &h.Completed,
		); err != nil {
			return nil, fmt.Errorf("scan habit: %w", err)
		}
		habits = append(habits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(habits) > 0 {
		ids := make([]int, len(habits))
		for i, h := range habits {
			ids[i] = h.ID
		}

		entryRows, err := s.db.Query(ctx,
			`SELECT habit_id, day FROM entries WHERE habit_id = ANY($1) AND day <= $2 ORDER BY habit_id, day`,
			ids, day)
		if err != nil {
			return nil, fmt.Errorf("fetch entries for streaks: %w", err)
		}
		defer entryRows.Close()

		entryMap := map[int][]time.Time{}
		for entryRows.Next() {
			var habitID int
			var d time.Time
			if err := entryRows.Scan(&habitID, &d); err != nil {
				return nil, fmt.Errorf("scan entry: %w", err)
			}
			entryMap[habitID] = append(entryMap[habitID], d)
		}
		if err := entryRows.Err(); err != nil {
			return nil, err
		}

		for i, h := range habits {
			habits[i].Streak = computeStreak(entryMap[h.ID], h.Period, h.Target, date)
		}
	}

	return habits, nil
}

func (s *HabitService) Create(ctx context.Context, name, description string, target int, period string) (model.Habit, error) {
	var h model.Habit
	err := s.db.QueryRow(ctx,
		`INSERT INTO habits (name, description, target, period) VALUES ($1, $2, $3, $4)
		 RETURNING id, name, description, target, period, created_at`,
		name, description, target, period,
	).Scan(&h.ID, &h.Name, &h.Description, &h.Target, &h.Period, &h.CreatedAt)
	if err != nil {
		return h, fmt.Errorf("create habit: %w", err)
	}
	return h, nil
}

func (s *HabitService) Delete(ctx context.Context, id int) error {
	_, err := s.db.Exec(ctx, `UPDATE habits SET deleted_at = NOW() WHERE id = $1`, id)
	return err
}

// Toggle adds or removes the entry for the habit on the given date.
// Returns true if an entry was created, false if it was removed.
func (s *HabitService) Toggle(ctx context.Context, habitID int, date time.Time) (hasEntry bool, err error) {
	day := date.Format("2006-01-02")

	var exists bool
	err = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM entries WHERE habit_id = $1 AND day = $2)`,
		habitID, day,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check entry: %w", err)
	}

	if exists {
		_, err = s.db.Exec(ctx, `DELETE FROM entries WHERE habit_id = $1 AND day = $2`, habitID, day)
		return false, err
	}

	_, err = s.db.Exec(ctx, `INSERT INTO entries (habit_id, day) VALUES ($1, $2)`, habitID, day)
	return true, err
}

