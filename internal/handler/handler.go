package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"habitual/internal/dateutil"
	"habitual/internal/model"
	"habitual/internal/service"
	"habitual/web/components"
)

type Handler struct {
	habits *service.HabitService
}

func New(habits *service.HabitService) *Handler {
	return &Handler{habits: habits}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("GET /day", h.day)
	mux.HandleFunc("GET /calendar", h.calendar)
	mux.HandleFunc("POST /habits", h.createHabit)
	mux.HandleFunc("POST /habits/{id}/toggle", h.toggleHabit)
	mux.HandleFunc("DELETE /habits/{id}", h.deleteHabit)
}

// parseDateParam reads a date from the request (query param or form value).
// Future dates are clamped to today.
func parseDateParam(r *http.Request) time.Time {
	today := dateutil.Today()

	for _, raw := range []string{r.URL.Query().Get("date"), r.FormValue("date")} {
		if raw == "" {
			continue
		}
		if t, err := dateutil.ParseDay(raw); err == nil {
			if !t.After(today) {
				return t
			}
		}
	}
	return today
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	date := parseDateParam(r)

	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries, err := h.habits.MonthSummary(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitsPage(habits, date, summaries).Render(r.Context(), w)
}

func (h *Handler) day(w http.ResponseWriter, r *http.Request) {
	date := parseDateParam(r)

	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries, err := h.habits.MonthSummary(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitsContent(habits, date, summaries).Render(r.Context(), w)
}

func (h *Handler) calendar(w http.ResponseWriter, r *http.Request) {
	monthStr := r.URL.Query().Get("month")
	var month time.Time
	if t, err := time.ParseInLocation("2006-01", monthStr, dateutil.Location()); err == nil {
		month = dateutil.FirstOfMonth(t)
	} else {
		month = dateutil.FirstOfMonth(dateutil.Today())
	}

	// Clamp to current month — don't allow future months
	currentMonth := dateutil.FirstOfMonth(dateutil.Today())
	if month.After(currentMonth) {
		month = currentMonth
	}

	selectedDate := parseDateParam(r)

	summaries, err := h.habits.MonthSummary(r.Context(), month)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.CalendarGrid(month, summaries, selectedDate).Render(r.Context(), w)
}

func (h *Handler) createHabit(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	description := r.FormValue("description")

	target, period := 1, "day"
	if parts := strings.SplitN(r.FormValue("frequency"), ":", 2); len(parts) == 2 {
		if n, err := strconv.Atoi(parts[0]); err == nil && n > 0 {
			target = n
		}
		if parts[1] == "week" || parts[1] == "month" {
			period = parts[1]
		}
	}

	if _, err := h.habits.Create(r.Context(), name, description, target, period); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := parseDateParam(r)
	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitList(habits, date).Render(r.Context(), w)
}

func (h *Handler) toggleHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := parseDateParam(r)

	if _, err := h.habits.Toggle(r.Context(), id, date); err != nil {
		switch {
		case errors.Is(err, service.ErrHabitNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, service.ErrHabitInactiveOnDate):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var toggled model.Habit
	for _, hab := range habits {
		if hab.ID == id {
			toggled = hab
			break
		}
	}

	isToday := dateutil.SameDay(date, dateutil.Today())
	if isToday && len(habits) > 0 && allCompleted(habits) {
		w.Header().Set("HX-Trigger", "confetti")
	}

	components.ToggleResponse(toggled, habits, date).Render(r.Context(), w)
}

func (h *Handler) deleteHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := parseDateParam(r)

	if err := h.habits.Delete(r.Context(), id, date); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.DeleteResponse(habits, date).Render(r.Context(), w)
}

func allCompleted(habits []model.Habit) bool {
	for _, h := range habits {
		if !h.Completed {
			return false
		}
	}
	return true
}
