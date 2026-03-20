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

func (s *HabitService) List(ctx context.Context) ([]model.Habit, error) {
	today := time.Now().Format("2006-01-02")

	rows, err := s.db.Query(ctx, `
		SELECT
			h.id, h.name, h.description, h.points, h.created_at,
			(e.id IS NOT NULL) AS completed_today,
			COALESCE((
				SELECT COUNT(*)
				FROM (
					SELECT day FROM entries
					WHERE habit_id = h.id
					  AND day >= CURRENT_DATE - (
						SELECT COUNT(*) - 1 FROM (
							SELECT day, LAG(day) OVER (ORDER BY day DESC) AS prev
							FROM entries WHERE habit_id = h.id ORDER BY day DESC
						) sub
						WHERE day - prev <= 1 OR prev IS NULL
						LIMIT 1
					  )
					ORDER BY day DESC
				) streak_days
			), 0) AS streak
		FROM habits h
		LEFT JOIN entries e ON e.habit_id = h.id AND e.day = $1
		ORDER BY h.created_at ASC
	`, today)
	if err != nil {
		return nil, fmt.Errorf("list habits: %w", err)
	}
	defer rows.Close()

	var habits []model.Habit
	for rows.Next() {
		var h model.Habit
		if err := rows.Scan(&h.ID, &h.Name, &h.Description, &h.Points, &h.CreatedAt, &h.CompletedToday, &h.Streak); err != nil {
			return nil, fmt.Errorf("scan habit: %w", err)
		}
		habits = append(habits, h)
	}
	return habits, rows.Err()
}

func (s *HabitService) Create(ctx context.Context, name, description string, points int) (model.Habit, error) {
	var h model.Habit
	err := s.db.QueryRow(ctx,
		`INSERT INTO habits (name, description, points) VALUES ($1, $2, $3)
		 RETURNING id, name, description, points, created_at`,
		name, description, points,
	).Scan(&h.ID, &h.Name, &h.Description, &h.Points, &h.CreatedAt)
	if err != nil {
		return h, fmt.Errorf("create habit: %w", err)
	}
	return h, nil
}

func (s *HabitService) Delete(ctx context.Context, id int) error {
	_, err := s.db.Exec(ctx, `DELETE FROM habits WHERE id = $1`, id)
	return err
}

// Toggle marks a habit complete for today, or removes the entry if already done.
func (s *HabitService) Toggle(ctx context.Context, habitID int) (completed bool, err error) {
	today := time.Now().Format("2006-01-02")

	var exists bool
	err = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM entries WHERE habit_id = $1 AND day = $2)`,
		habitID, today,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check entry: %w", err)
	}

	if exists {
		_, err = s.db.Exec(ctx, `DELETE FROM entries WHERE habit_id = $1 AND day = $2`, habitID, today)
		return false, err
	}

	_, err = s.db.Exec(ctx, `INSERT INTO entries (habit_id, day) VALUES ($1, $2)`, habitID, today)
	return true, err
}

// TodayPoints returns total points earned today.
func (s *HabitService) TodayPoints(ctx context.Context) (int, error) {
	today := time.Now().Format("2006-01-02")
	var total int
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(h.points), 0)
		FROM entries e
		JOIN habits h ON h.id = e.habit_id
		WHERE e.day = $1
	`, today).Scan(&total)
	return total, err
}
