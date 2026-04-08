package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"habitual/internal/model"

	"github.com/stretchr/testify/require"
)

func TestHabitRowTogglesFromRowButNotDeleteButton(t *testing.T) {
	var out bytes.Buffer
	habit := model.Habit{
		ID:          42,
		Name:        "Read",
		Description: "Read for 30 minutes",
		Target:      1,
		Period:      "day",
	}
	date := time.Date(2026, time.April, 8, 0, 0, 0, 0, time.UTC)

	err := HabitRow(habit, date).Render(context.Background(), &out)
	require.NoError(t, err)

	html := out.String()

	require.Contains(t, html, `class="habit-row"`)
	require.Contains(t, html, `id="habit-42"`)
	require.Contains(t, html, `role="button"`)
	require.Contains(t, html, `tabindex="0"`)
	require.Contains(t, html, `aria-label="Toggle habit Read"`)
	require.Contains(t, html, `hx-post="/habits/42/toggle?date=2026-04-08"`)
	require.Contains(t, html, `hx-target="#habit-42"`)
	require.Contains(t, html, `hx-swap="outerHTML"`)
	require.Contains(t, html, `hx-trigger="click, keyup[key=='Enter'], keyup[key==' ']"`)
	require.Contains(t, html, `<span class="toggle-btn" aria-hidden="true">`)
	require.Contains(t, html, `<button class="delete-btn" hx-delete="/habits/42?date=2026-04-08" hx-target="#habit-42" hx-swap="outerHTML" hx-confirm="Delete this habit?" hx-on:click="event.stopPropagation()">`)
	require.NotContains(t, html, `<button class="toggle-btn"`)
	require.Equal(t, 1, strings.Count(html, `hx-post="/habits/42/toggle?date=2026-04-08"`))
}
