package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"ns116/internal/database"
)

const (
	cookieName    = "ns116_session"
	sessionMaxAge = 24 * time.Hour
)

type SessionManager struct {
	secret string
	db     *database.DB
}

func NewSessionManager(db *database.DB) (*SessionManager, error) {
	secret, err := db.EnsureSessionSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to load session secret: %w", err)
	}
	return &SessionManager{secret: secret, db: db}, nil
}

func (sm *SessionManager) CreateSession(w http.ResponseWriter, username string) string {
	token := generateToken()
	csrfToken := generateToken()
	signed := sm.sign(token)
	expiresAt := time.Now().Add(sessionMaxAge)

	_ = sm.db.CreateSession(signed, csrfToken, username, expiresAt)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
	return csrfToken
}

func (sm *SessionManager) DestroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err == nil {
		_ = sm.db.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func (sm *SessionManager) GetSessionInfo(r *http.Request) (string, string, bool) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return "", "", false
	}
	username, csrfToken, expiresAt, err := sm.db.GetSession(cookie.Value)
	if err != nil || username == "" || time.Now().After(expiresAt) {
		return "", "", false
	}
	return username, csrfToken, true
}

func (sm *SessionManager) GetUsername(r *http.Request) (string, bool) {
	username, _, ok := sm.GetSessionInfo(r)
	return username, ok
}

func (sm *SessionManager) ValidateCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
			_, csrfToken, ok := sm.GetSessionInfo(r)
			if !ok {
				http.Error(w, "Forbidden: No session", http.StatusForbidden)
				return
			}

			submitted := r.FormValue("csrf_token")
			if submitted == "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}

			if submitted == "" || submitted != csrfToken {
				http.Error(w, "Forbidden: Invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

func (sm *SessionManager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := sm.GetUsername(r); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (sm *SessionManager) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, ok := sm.GetUsername(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, _ := sm.db.GetUserByUsername(username)
		if user == nil || user.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (sm *SessionManager) sign(token string) string {
	mac := hmac.New(sha256.New, []byte(sm.secret))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func generateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
