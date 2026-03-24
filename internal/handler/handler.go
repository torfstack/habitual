package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"habitual/internal/auth"
	"habitual/internal/dateutil"
	"habitual/internal/model"
	"habitual/internal/service"
	"habitual/web/components"
)

type Handler struct {
	habits *service.HabitService
	auth   *auth.Service
}

func New(habits *service.HabitService, authSvc *auth.Service) *Handler {
	return &Handler{habits: habits, auth: authSvc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("GET /auth/google", h.startGoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", h.googleCallback)
	mux.HandleFunc("POST /logout", h.logout)

	mux.Handle("GET /", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.index))))
	mux.Handle("GET /day", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.day))))
	mux.Handle("GET /calendar", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.calendar))))
	mux.Handle("POST /habits", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.createHabit))))
	mux.Handle("POST /habits/{id}/toggle", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.toggleHabit))))
	mux.Handle("DELETE /habits/{id}", h.withCurrentUser(h.requireAuth(http.HandlerFunc(h.deleteHabit))))
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

func (h *Handler) withCurrentUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cookie, err := r.Cookie(auth.SessionCookieName)
		if err == nil && cookie.Value != "" {
			user, userErr := h.auth.CurrentUser(ctx, cookie.Value)
			if userErr == nil {
				ctx = auth.ContextWithUser(ctx, user)
			} else if !errors.Is(userErr, auth.ErrSessionNotFound) {
				log.Printf("load current user: %v", userErr)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.UserFromContext(r.Context()) == nil {
			redirectToLogin(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
		if user, userErr := h.auth.CurrentUser(ctx, cookie.Value); userErr == nil && user != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	components.LoginPage(h.auth.Enabled()).Render(ctx, w)
}

func (h *Handler) startGoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "unable to start login", http.StatusInternalServerError)
		return
	}

	url, err := h.auth.AuthCodeURL(state)
	if err != nil {
		if errors.Is(err, auth.ErrAuthNotConfigured) {
			components.LoginPage(false).Render(r.Context(), w)
			return
		}
		http.Error(w, "unable to start login", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.StateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   10 * 60,
	})

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(auth.StateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, auth.ErrInvalidState.Error(), http.StatusBadRequest)
		return
	}

	expireCookie(w, auth.StateCookieName, isSecureRequest(r))

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing auth code", http.StatusBadRequest)
		return
	}

	_, sessionToken, err := h.auth.HandleGoogleCallback(r.Context(), code)
	if err != nil {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		if err := h.auth.DeleteSession(r.Context(), cookie.Value); err != nil {
			log.Printf("delete session: %v", err)
		}
	}

	expireCookie(w, auth.SessionCookieName, isSecureRequest(r))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	date := parseDateParam(r)

	habits, err := h.habits.List(r.Context(), user.ID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries, err := h.habits.MonthSummary(r.Context(), user.ID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitsPage(user, habits, date, summaries).Render(r.Context(), w)
}

func (h *Handler) day(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	date := parseDateParam(r)

	habits, err := h.habits.List(r.Context(), user.ID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summaries, err := h.habits.MonthSummary(r.Context(), user.ID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitsContent(habits, date, summaries).Render(r.Context(), w)
}

func (h *Handler) calendar(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	monthStr := r.URL.Query().Get("month")
	var month time.Time
	if t, err := time.ParseInLocation("2006-01", monthStr, dateutil.Location()); err == nil {
		month = dateutil.FirstOfMonth(t)
	} else {
		month = dateutil.FirstOfMonth(dateutil.Today())
	}

	currentMonth := dateutil.FirstOfMonth(dateutil.Today())
	if month.After(currentMonth) {
		month = currentMonth
	}

	selectedDate := parseDateParam(r)

	summaries, err := h.habits.MonthSummary(r.Context(), user.ID, month)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.CalendarGrid(month, summaries, selectedDate).Render(r.Context(), w)
}

func (h *Handler) createHabit(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
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

	if _, err := h.habits.Create(r.Context(), user.ID, name, description, target, period); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := parseDateParam(r)
	habits, err := h.habits.List(r.Context(), user.ID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	components.HabitList(habits, date).Render(r.Context(), w)
}

func (h *Handler) toggleHabit(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := parseDateParam(r)

	if _, err := h.habits.Toggle(r.Context(), user.ID, id, date); err != nil {
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

	habits, err := h.habits.List(r.Context(), user.ID, date)
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
	user := auth.UserFromContext(r.Context())
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	date := parseDateParam(r)

	if err := h.habits.Delete(r.Context(), user.ID, id, date); err != nil {
		if errors.Is(err, service.ErrHabitNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	habits, err := h.habits.List(r.Context(), user.ID, date)
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

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func expireCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func randomState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
