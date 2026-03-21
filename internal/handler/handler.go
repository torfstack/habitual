package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"habitual/internal/service"
	"habitual/web/components"
)

const dateLayout = "2006-01-02"

type Handler struct {
	habits *service.HabitService
}

func New(habits *service.HabitService) *Handler {
	return &Handler{habits: habits}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("POST /habits", h.createHabit)
	mux.HandleFunc("POST /habits/{id}/toggle", h.toggleHabit)
	mux.HandleFunc("DELETE /habits/{id}", h.deleteHabit)
}

// parseDateParam reads a date from the request (query param or form value).
// Future dates are clamped to today.
func parseDateParam(r *http.Request) time.Time {
	today := time.Now().Truncate(24 * time.Hour)

	for _, raw := range []string{r.URL.Query().Get("date"), r.FormValue("date")} {
		if raw == "" {
			continue
		}
		if t, err := time.Parse(dateLayout, raw); err == nil {
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

	components.HabitsPage(habits, date).Render(r.Context(), w)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habits, err := h.habits.List(r.Context(), date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitList(habits, date).Render(r.Context(), w)
}

func (h *Handler) deleteHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.habits.Delete(r.Context(), id); err != nil {
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
