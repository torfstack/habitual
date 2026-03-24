package dateutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDayUsesLocalCalendarDate(t *testing.T) {
	parsed, err := ParseDay("2026-03-24")
	require.NoError(t, err)
	require.Equal(t, Location(), parsed.Location())
	require.Equal(t, 2026, parsed.Year())
	require.Equal(t, time.March, parsed.Month())
	require.Equal(t, 24, parsed.Day())
}

func TestStartOfDayUsesLocalMidnight(t *testing.T) {
	input := time.Date(2026, 3, 24, 18, 45, 0, 0, time.FixedZone("offset", 3*3600))
	day := StartOfDay(input)

	require.Equal(t, Location(), day.Location())
	require.Equal(t, 0, day.Hour())
	require.Equal(t, 0, day.Minute())
	require.Equal(t, 0, day.Second())
}
