package components

import (
	"fmt"
	"time"

	"habitual/internal/model"
)


func formatDateLabel(t time.Time) string {
	today := time.Now().Truncate(24 * time.Hour)
	d := t.Truncate(24 * time.Hour)
	switch {
	case d.Equal(today):
		return "Today"
	case d.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return d.Format("Mon, Jan 2")
	}
}

func isToday(t time.Time) bool {
	today := time.Now().Truncate(24 * time.Hour)
	return t.Truncate(24 * time.Hour).Equal(today)
}

func dateParam(t time.Time) string {
	return t.Format("2006-01-02")
}

func allCompleted(habits []model.Habit) bool {
	for _, h := range habits {
		if !h.Completed {
			return false
		}
	}
	return true
}

func freqLabel(target int, period string) string {
	if target == 1 {
		switch period {
		case "week":
			return "1× / wk"
		case "month":
			return "1× / mo"
		}
		return ""
	}
	switch period {
	case "week":
		return fmt.Sprintf("%d× / wk", target)
	case "month":
		return fmt.Sprintf("%d× / mo", target)
	}
	return ""
}
