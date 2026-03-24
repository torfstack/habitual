package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"habitual/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	SessionCookieName = "habitual_session"
	StateCookieName   = "habitual_oauth_state"
)

var (
	ErrAuthNotConfigured = errors.New("google auth is not configured")
	ErrInvalidState      = errors.New("invalid oauth state")
	ErrSessionNotFound   = errors.New("session not found")
)

type Service struct {
	db          *pgxpool.Pool
	oauthConfig *oauth2.Config
	httpClient  *http.Client
	sessionTTL  time.Duration
}

type googleUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func NewService(db *pgxpool.Pool) *Service {
	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	redirectURL := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL"))

	var oauthConfig *oauth2.Config
	if clientID != "" && clientSecret != "" && redirectURL != "" {
		oauthConfig = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "profile", "email"},
		}
	}

	return &Service{
		db:          db,
		oauthConfig: oauthConfig,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		sessionTTL:  30 * 24 * time.Hour,
	}
}

func (s *Service) Enabled() bool {
	return s.oauthConfig != nil
}

func (s *Service) AuthCodeURL(state string) (string, error) {
	if !s.Enabled() {
		return "", ErrAuthNotConfigured
	}
	return s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (s *Service) HandleGoogleCallback(ctx context.Context, code string) (model.User, string, error) {
	if !s.Enabled() {
		return model.User{}, "", ErrAuthNotConfigured
	}

	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return model.User{}, "", fmt.Errorf("exchange code: %w", err)
	}

	userInfo, err := s.fetchGoogleUser(ctx, token.AccessToken)
	if err != nil {
		return model.User{}, "", err
	}

	user, err := s.upsertGoogleUser(ctx, userInfo)
	if err != nil {
		return model.User{}, "", err
	}

	sessionToken, err := randomToken(32)
	if err != nil {
		return model.User{}, "", fmt.Errorf("generate session token: %w", err)
	}

	if err := s.createSession(ctx, user.ID, sessionToken); err != nil {
		return model.User{}, "", err
	}

	return user, sessionToken, nil
}

func (s *Service) CurrentUser(ctx context.Context, sessionToken string) (*model.User, error) {
	if sessionToken == "" {
		return nil, ErrSessionNotFound
	}

	var user model.User
	err := s.db.QueryRow(
		ctx, `
		SELECT u.id, u.google_sub, u.email, u.name, u.picture_url, u.created_at, u.last_login_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.expires_at > NOW()
	`, hashToken(sessionToken),
	).Scan(
		&user.ID, &user.GoogleSub, &user.Email, &user.Name, &user.PictureURL, &user.CreatedAt, &user.LastLoginAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("load current user: %w", err)
	}

	return &user, nil
}

func (s *Service) DeleteSession(ctx context.Context, sessionToken string) error {
	if sessionToken == "" {
		return nil
	}
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(sessionToken))
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Service) fetchGoogleUser(ctx context.Context, accessToken string) (googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openidconnect.googleapis.com/v1/userinfo", nil)
	if err != nil {
		return googleUserInfo{}, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return googleUserInfo{}, fmt.Errorf("fetch google userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return googleUserInfo{}, fmt.Errorf(
			"fetch google userinfo: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)),
		)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return googleUserInfo{}, fmt.Errorf("decode google userinfo: %w", err)
	}
	if info.Sub == "" || info.Email == "" || info.Name == "" {
		return googleUserInfo{}, fmt.Errorf("google userinfo missing required fields")
	}

	return info, nil
}

func (s *Service) upsertGoogleUser(ctx context.Context, info googleUserInfo) (model.User, error) {
	var user model.User
	err := s.db.QueryRow(
		ctx, `
		INSERT INTO users (google_sub, email, name, picture_url, last_login_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (google_sub) DO UPDATE
		SET email = EXCLUDED.email,
		    name = EXCLUDED.name,
		    picture_url = EXCLUDED.picture_url,
		    last_login_at = NOW()
		RETURNING id, google_sub, email, name, picture_url, created_at, last_login_at
	`, info.Sub, info.Email, info.Name, info.Picture,
	).Scan(
		&user.ID, &user.GoogleSub, &user.Email, &user.Name, &user.PictureURL, &user.CreatedAt, &user.LastLoginAt,
	)
	if err != nil {
		return model.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

func (s *Service) createSession(ctx context.Context, userID int, sessionToken string) error {
	_, err := s.db.Exec(
		ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashToken(sessionToken), time.Now().Add(s.sessionTTL),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
