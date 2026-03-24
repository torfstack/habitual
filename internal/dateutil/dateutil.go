package dateutil

import "time"

const DayLayout = "2006-01-02"

func Location() *time.Location {
	return time.Local
}

func StartOfDay(t time.Time) time.Time {
	local := t.In(Location())
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Location())
}

func Today() time.Time {
	return StartOfDay(time.Now())
}

func ParseDay(raw string) (time.Time, error) {
	return time.ParseInLocation(DayLayout, raw, Location())
}

func SameDay(a, b time.Time) bool {
	aa := StartOfDay(a)
	bb := StartOfDay(b)
	return aa.Equal(bb)
}

func FirstOfMonth(t time.Time) time.Time {
	local := t.In(Location())
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, Location())
}
