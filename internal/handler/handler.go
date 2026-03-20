package handler

import (
	"net/http"
	"strconv"

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
	mux.HandleFunc("POST /habits", h.createHabit)
	mux.HandleFunc("POST /habits/{id}/toggle", h.toggleHabit)
	mux.HandleFunc("DELETE /habits/{id}", h.deleteHabit)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	habits, err := h.habits.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	points, err := h.habits.TodayPoints(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitsPage(habits, points).Render(r.Context(), w)
}

func (h *Handler) createHabit(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	description := r.FormValue("description")
	points, _ := strconv.Atoi(r.FormValue("points"))
	if points <= 0 {
		points = 1
	}

	if _, err := h.habits.Create(r.Context(), name, description, points); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habits, err := h.habits.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitList(habits).Render(r.Context(), w)
}

func (h *Handler) toggleHabit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if _, err := h.habits.Toggle(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habits, err := h.habits.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Find the updated habit and return just its row
	for _, habit := range habits {
		if habit.ID == id {
			components.HabitRow(habit).Render(r.Context(), w)
			return
		}
	}

	http.Error(w, "habit not found", http.StatusNotFound)
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

	habits, err := h.habits.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitList(habits).Render(r.Context(), w)
}
