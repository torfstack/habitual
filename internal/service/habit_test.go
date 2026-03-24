package service_test

import (
	"context"
	"testing"
	"time"

	"habitual/internal/db"
	"habitual/internal/model"
	"habitual/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func findHabit(habits []model.Habit, id int) *model.Habit {
	for i := range habits {
		if habits[i].ID == id {
			return &habits[i]
		}
	}
	return nil
}

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgc, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("habitual"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := pgc.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	connStr, err := pgc.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool))
	return pool
}

func setHabitCreatedAt(t *testing.T, pool *pgxpool.Pool, habitID int, createdAt time.Time) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`UPDATE habits SET created_at = $2 WHERE id = $1`,
		habitID, createdAt,
	)
	require.NoError(t, err)
}

func TestHabitService(t *testing.T) {
	pool := setupDB(t)
	svc := service.NewHabitService(pool)
	ctx := context.Background()
	today := time.Now()
	yesterday := today.AddDate(0, 0, -1)

	t.Run("Create", func(t *testing.T) {
		h, err := svc.Create(ctx, "Read", "Read for 30 minutes", 1, "day")
		require.NoError(t, err)
		assert.NotZero(t, h.ID)
		assert.Equal(t, "Read", h.Name)
		assert.Equal(t, "Read for 30 minutes", h.Description)
		assert.Equal(t, 1, h.Target)
		assert.Equal(t, "day", h.Period)
		assert.False(t, h.CreatedAt.IsZero())
	})

	t.Run("List", func(t *testing.T) {
		habits, err := svc.List(ctx, today)
		require.NoError(t, err)
		assert.NotEmpty(t, habits)
		for _, h := range habits {
			assert.NotZero(t, h.ID)
			assert.NotEmpty(t, h.Name)
		}
	})

	t.Run("Toggle complete and uncomplete", func(t *testing.T) {
		h, err := svc.Create(ctx, "Exercise", "", 1, "day")
		require.NoError(t, err)

		hasEntry, err := svc.Toggle(ctx, h.ID, today)
		require.NoError(t, err)
		assert.True(t, hasEntry)

		habits, err := svc.List(ctx, today)
		require.NoError(t, err)
		var found bool
		for _, habit := range habits {
			if habit.ID == h.ID {
				assert.True(t, habit.HasEntry)
				assert.True(t, habit.Completed)
				found = true
			}
		}
		assert.True(t, found)

		hasEntry, err = svc.Toggle(ctx, h.ID, today)
		require.NoError(t, err)
		assert.False(t, hasEntry)
	})

	t.Run("Delete hides habit from today", func(t *testing.T) {
		h, err := svc.Create(ctx, "Temporary", "", 1, "day")
		require.NoError(t, err)

		err = svc.Delete(ctx, h.ID, today)
		require.NoError(t, err)

		habits, err := svc.List(ctx, today)
		require.NoError(t, err)
		for _, habit := range habits {
			assert.NotEqual(t, h.ID, habit.ID, "deleted habit should not appear for today")
		}
	})

	t.Run("Delete preserves habit in past dates", func(t *testing.T) {
		h, err := svc.Create(ctx, "Was active yesterday", "", 1, "day")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, yesterday.AddDate(0, 0, -1))

		_, err = svc.Toggle(ctx, h.ID, yesterday)
		require.NoError(t, err)

		err = svc.Delete(ctx, h.ID, today)
		require.NoError(t, err)

		// should be gone from today
		todayHabits, err := svc.List(ctx, today)
		require.NoError(t, err)
		for _, habit := range todayHabits {
			assert.NotEqual(t, h.ID, habit.ID, "deleted habit should not appear for today")
		}

		// should still appear for yesterday
		pastHabits, err := svc.List(ctx, yesterday)
		require.NoError(t, err)
		var found bool
		for _, habit := range pastHabits {
			if habit.ID == h.ID {
				found = true
				assert.True(t, habit.HasEntry)
			}
		}
		assert.True(t, found, "deleted habit should still appear for past dates")
	})

	t.Run("Weekly habit period_count and completion", func(t *testing.T) {
		h, err := svc.Create(ctx, "Gym", "", 2, "week")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, yesterday.AddDate(0, 0, -1))

		// one entry — not yet complete for the week
		_, err = svc.Toggle(ctx, h.ID, today)
		require.NoError(t, err)

		habits, err := svc.List(ctx, today)
		require.NoError(t, err)
		for _, habit := range habits {
			if habit.ID == h.ID {
				assert.Equal(t, 1, habit.PeriodCount)
				assert.False(t, habit.Completed)
				assert.True(t, habit.HasEntry)
			}
		}

		// second entry on a different day this week (yesterday)
		_, err = svc.Toggle(ctx, h.ID, yesterday)
		require.NoError(t, err)

		// both days must be in the same week for this to pass;
		// if today is Monday, yesterday is in last week — skip.
		if today.Weekday() != time.Monday {
			habits, err = svc.List(ctx, today)
			require.NoError(t, err)
			for _, habit := range habits {
				if habit.ID == h.ID {
					assert.Equal(t, 2, habit.PeriodCount)
					assert.True(t, habit.Completed)
				}
			}
		}
	})

	t.Run("Streak increments on consecutive days", func(t *testing.T) {
		h, err := svc.Create(ctx, "Streak habit", "", 1, "day")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, yesterday.AddDate(0, 0, -1))

		// no entries yet — streak should be 0
		habits, err := svc.List(ctx, today)
		require.NoError(t, err)
		if found := findHabit(habits, h.ID); assert.NotNil(t, found) {
			assert.Equal(t, 0, found.Streak)
		}

		// check in for yesterday and today
		_, err = svc.Toggle(ctx, h.ID, yesterday)
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h.ID, today)
		require.NoError(t, err)

		habits, err = svc.List(ctx, today)
		require.NoError(t, err)
		if found := findHabit(habits, h.ID); assert.NotNil(t, found) {
			assert.Equal(t, 2, found.Streak, "streak should be 2 after two consecutive days")
		}

		// streak of 1 (only today)
		h2, err := svc.Create(ctx, "Day-one streak", "", 1, "day")
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h2.ID, today)
		require.NoError(t, err)

		habits, err = svc.List(ctx, today)
		require.NoError(t, err)
		if found := findHabit(habits, h2.ID); assert.NotNil(t, found) {
			assert.Equal(t, 1, found.Streak, "streak should be 1 on first completion")
		}
	})

	t.Run("Weekly streak: in-progress week does not break past streak", func(t *testing.T) {
		h, err := svc.Create(ctx, "Gym weekly", "", 3, "week")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

		// Use fixed dates: ref = Wednesday 2025-03-19
		ref := time.Date(2025, 3, 19, 0, 0, 0, 0, time.UTC)

		// Week 2025-03-03..09: 3 entries (completed)
		for _, d := range []int{3, 4, 5} {
			_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, d, 0, 0, 0, 0, time.UTC))
			require.NoError(t, err)
		}
		// Week 2025-03-10..16: 3 entries (completed)
		for _, d := range []int{10, 11, 12} {
			_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, d, 0, 0, 0, 0, time.UTC))
			require.NoError(t, err)
		}
		// Week 2025-03-17..23 (current): only 1 entry (in progress)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		habits, err := svc.List(ctx, ref)
		require.NoError(t, err)
		found := findHabit(habits, h.ID)
		require.NotNil(t, found)
		assert.Equal(t, 2, found.Streak, "streak should be 2 (two completed weeks); in-progress week does not break it")
		assert.Equal(t, 1, found.PeriodCount)
		assert.False(t, found.Completed)
	})

	t.Run("Weekly streak includes current week when completed", func(t *testing.T) {
		h, err := svc.Create(ctx, "Gym current week", "", 2, "week")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))

		ref := time.Date(2025, 3, 19, 0, 0, 0, 0, time.UTC)

		// Last week: 2 entries (completed)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 11, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		// Current week: 2 entries (completed)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 17, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		habits, err := svc.List(ctx, ref)
		require.NoError(t, err)
		found := findHabit(habits, h.ID)
		require.NotNil(t, found)
		assert.Equal(t, 2, found.Streak, "streak should be 2: current week + last week both completed")
		assert.True(t, found.Completed)
	})

	t.Run("Monthly streak: in-progress month does not break past streak", func(t *testing.T) {
		h, err := svc.Create(ctx, "Monthly review", "", 2, "month")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

		ref := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

		// January: 2 entries (completed)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		// February: 2 entries (completed)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		// March: 1 entry (in progress)
		_, err = svc.Toggle(ctx, h.ID, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		habits, err := svc.List(ctx, ref)
		require.NoError(t, err)
		found := findHabit(habits, h.ID)
		require.NotNil(t, found)
		assert.Equal(t, 2, found.Streak, "streak should be 2 (Jan + Feb completed); in-progress March does not break it")
		assert.Equal(t, 1, found.PeriodCount)
		assert.False(t, found.Completed)
	})

	t.Run("History: past date shows past entry", func(t *testing.T) {
		h, err := svc.Create(ctx, "History habit", "", 1, "day")
		require.NoError(t, err)
		setHabitCreatedAt(t, pool, h.ID, yesterday.AddDate(0, 0, -1))

		_, err = svc.Toggle(ctx, h.ID, yesterday)
		require.NoError(t, err)

		pastHabits, err := svc.List(ctx, yesterday)
		require.NoError(t, err)
		for _, habit := range pastHabits {
			if habit.ID == h.ID {
				assert.True(t, habit.HasEntry)
				assert.True(t, habit.Completed)
			}
		}

		// today's view should not show it as completed
		todayHabits, err := svc.List(ctx, today)
		require.NoError(t, err)
		for _, habit := range todayHabits {
			if habit.ID == h.ID {
				assert.False(t, habit.HasEntry)
				assert.False(t, habit.Completed)
			}
		}
	})
}
