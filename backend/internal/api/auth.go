package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"playhld/backend/internal/identity"
	"playhld/backend/internal/storage"
)

const sessionCookieName = "sf_session"
const csrfCookieName = "sf_csrf"
const sessionLifetime = 8 * time.Hour

type actor struct {
	ID        string
	Email     string
	CSRFHash  []byte
	TokenHash [32]byte
}
type authInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func readStrict(w http.ResponseWriter, r *http.Request, max int64, target any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "send application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large")
		} else {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is invalid")
		}
		return false
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "multiple JSON values")
		return false
	}
	return true
}

func sameOrigin(r *http.Request) bool {
	value := r.Header.Get("Origin")
	if value == "" {
		return true
	}
	origin, err := url.Parse(value)
	return err == nil && (origin.Scheme == "http" || origin.Scheme == "https") && strings.EqualFold(origin.Host, r.Host)
}

func randomToken() (string, [32]byte, error) {
	var hash [32]byte
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", hash, err
	}
	hash = sha256.Sum256(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes), hash, nil
}

func setSessionCookies(w http.ResponseWriter, token, csrf string, secure bool) {
	common := http.Cookie{Path: "/", MaxAge: int(sessionLifetime.Seconds()), SameSite: http.SameSiteStrictMode, Secure: secure}
	session := common
	session.Name = sessionCookieName
	session.Value = token
	session.HttpOnly = true
	csrfCookie := common
	csrfCookie.Name = csrfCookieName
	csrfCookie.Value = csrf
	http.SetCookie(w, &session)
	http.SetCookie(w, &csrfCookie)
}

func clearSessionCookies(w http.ResponseWriter, secure bool) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, SameSite: http.SameSiteStrictMode, Secure: secure, HttpOnly: name == sessionCookieName})
	}
}

func authenticate(ctx context.Context, db *storage.DB, r *http.Request) (actor, error) {
	var user actor
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return user, err
	}
	bytes, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(bytes) != 32 {
		return user, errors.New("invalid session")
	}
	user.TokenHash = sha256.Sum256(bytes)
	err = db.QueryRow(ctx, `SELECT u.id,u.email,s.csrf_hash FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>CURRENT_TIMESTAMP`, user.TokenHash[:]).Scan(&user.ID, &user.Email, &user.CSRFHash)
	return user, err
}

func requireActor(w http.ResponseWriter, r *http.Request, db *storage.DB, csrf bool) (actor, bool) {
	user, err := authenticate(r.Context(), db, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "sign in to continue")
		return actor{}, false
	}
	if csrf {
		cookie, cookieErr := r.Cookie(csrfCookieName)
		header := r.Header.Get("X-CSRF-Token")
		cookieValue := ""
		if cookieErr == nil {
			cookieValue = cookie.Value
		}
		cookieBytes, cookieDecodeErr := base64.RawURLEncoding.DecodeString(cookieValue)
		headerBytes, headerDecodeErr := base64.RawURLEncoding.DecodeString(header)
		if cookieErr != nil || cookieDecodeErr != nil || headerDecodeErr != nil || len(cookieBytes) != 32 || len(headerBytes) != 32 || subtle.ConstantTimeCompare(cookieBytes, headerBytes) != 1 {
			writeError(w, http.StatusForbidden, "CSRF_FAILED", "request token is missing or invalid")
			return actor{}, false
		}
		hash := sha256.Sum256(headerBytes)
		if subtle.ConstantTimeCompare(hash[:], user.CSRFHash) != 1 {
			writeError(w, http.StatusForbidden, "CSRF_FAILED", "request token is missing or invalid")
			return actor{}, false
		}
	}
	return user, true
}

type attemptWindow struct {
	count  int
	starts time.Time
}
type authLimiter struct {
	mu       sync.Mutex
	attempts map[string]attemptWindow
}

func newAuthLimiter() *authLimiter { return &authLimiter{attempts: map[string]attemptWindow{}} }
func (l *authLimiter) allow(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.attempts) > 10_000 {
		l.attempts = map[string]attemptWindow{}
	}
	window := l.attempts[host]
	if now.Sub(window.starts) > 5*time.Minute {
		window = attemptWindow{starts: now}
	}
	window.count++
	l.attempts[host] = window
	return window.count <= 20
}

func registerAuthRoutes(mux *http.ServeMux, db *storage.DB, secure bool) {
	limiter := newAuthLimiter()
	dummyHash, _ := identity.HashPassword("dummy password for timing checks")
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		if !limiter.allow(r.RemoteAddr) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "try again later")
			return
		}
		var input authInput
		if !readStrict(w, r, 4096, &input) {
			return
		}
		email, err := identity.NormalizeEmail(input.Email)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_EMAIL", err.Error())
			return
		}
		hash, err := identity.HashPassword(input.Password)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_PASSWORD", err.Error())
			return
		}
		token, tokenHash, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "could not create session")
			return
		}
		csrf, csrfHash, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "could not create session")
			return
		}
		tx, err := db.Begin(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not register")
			return
		}
		defer tx.Rollback(r.Context())
		var userID string
		err = tx.QueryRow(r.Context(), `INSERT INTO users(email,password_hash) VALUES($1,$2) RETURNING id`, email, hash).Scan(&userID)
		if err != nil {
			if storage.IsUniqueConstraint(err) {
				writeError(w, http.StatusConflict, "EMAIL_IN_USE", "email is already registered")
			} else {
				writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not register")
			}
			return
		}
		_, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,csrf_hash,expires_at) VALUES($1,$2,$3,$4)`, tokenHash[:], userID, csrfHash[:], time.Now().Add(sessionLifetime))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not create session")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not register")
			return
		}
		setSessionCookies(w, token, csrf, secure)
		writeJSON(w, http.StatusCreated, map[string]string{"id": userID, "email": email})
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		if !limiter.allow(r.RemoteAddr) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "try again later")
			return
		}
		var input authInput
		if !readStrict(w, r, 4096, &input) {
			return
		}
		email, err := identity.NormalizeEmail(input.Email)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
			return
		}
		var userID, passwordHash string
		err = db.QueryRow(r.Context(), `SELECT id,password_hash FROM users WHERE email=$1`, email).Scan(&userID, &passwordHash)
		if err != nil {
			passwordHash = dummyHash
		}
		valid := identity.VerifyPassword(passwordHash, input.Password)
		if err != nil || !valid {
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
			return
		}
		token, tokenHash, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "could not create session")
			return
		}
		csrf, csrfHash, err := randomToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "could not create session")
			return
		}
		_, err = db.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,csrf_hash,expires_at) VALUES($1,$2,$3,$4)`, tokenHash[:], userID, csrfHash[:], time.Now().Add(sessionLifetime))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not create session")
			return
		}
		setSessionCookies(w, token, csrf, secure)
		writeJSON(w, http.StatusOK, map[string]string{"id": userID, "email": email})
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": user.ID, "email": user.Email})
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		if _, err := db.Exec(r.Context(), `UPDATE sessions SET revoked_at=CURRENT_TIMESTAMP WHERE token_hash=$1`, user.TokenHash[:]); err != nil {
			writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not sign out")
			return
		}
		clearSessionCookies(w, secure)
		writeJSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
	})
}
