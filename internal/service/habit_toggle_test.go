package service_test

import (
	"context"
	"testing"

	"habitual/internal/service"

	"github.com/stretchr/testify/require"
)

func TestToggleRejectsDatesOutsideHabitLifetime(t *testing.T) {
	pool := setupDB(t)
	svc := service.NewHabitService(pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, "Read", "", 1, "day")
	require.NoError(t, err)

	beforeCreate := created.CreatedAt.AddDate(0, 0, -1)
	_, err = svc.Toggle(ctx, created.ID, beforeCreate)
	require.ErrorIs(t, err, service.ErrHabitInactiveOnDate)

	deleteDate := created.CreatedAt.AddDate(0, 0, 2)
	err = svc.Delete(ctx, created.ID, deleteDate)
	require.NoError(t, err)

	_, err = svc.Toggle(ctx, created.ID, deleteDate)
	require.ErrorIs(t, err, service.ErrHabitInactiveOnDate)

	_, err = svc.Toggle(ctx, created.ID, deleteDate.AddDate(0, 0, 1))
	require.ErrorIs(t, err, service.ErrHabitInactiveOnDate)
}
