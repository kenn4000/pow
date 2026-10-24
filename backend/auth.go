package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"prisonersofwar/profile"
)

type account struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("invalid-password-placeholder"), bcrypt.DefaultCost)

func randomID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data[:])
}

func normalizeEmail(input string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input))
	parsed, err := mail.ParseAddress(email)
	if err != nil || len(email) > 254 || parsed.Address != email {
		return "", errors.New("enter a valid email address")
	}
	return email, nil
}

func validPassword(password string) bool {
	return len(password) >= 12 && len(password) <= 72
}

func (s *webServer) currentAccount(r *http.Request) (account, error) {
	var user account
	cookie, err := r.Cookie("pow_session")
	if err != nil || len(cookie.Value) > 100 {
		return user, sql.ErrNoRows
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	err = s.db.QueryRowContext(r.Context(), `SELECT u.id,u.email,u.role,u.created_at FROM auth_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()`, hash[:]).Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt)
	return user, err
}

func (s *webServer) requireAccount(w http.ResponseWriter, r *http.Request) (account, bool) {
	if u, ok := r.Context().Value(accountKey).(account); ok {
		return u, true
	}
	user, err := s.currentAccount(r)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, errors.New("sign in to continue"))
		return user, false
	}
	if err != nil {
		databaseError(w, err)
		return user, false
	}
	return user, true
}

func databaseError(w http.ResponseWriter, err error) {
	fmt.Fprintf(os.Stderr, "database request failed: %v\n", err)
	writeError(w, http.StatusInternalServerError, errors.New("storage unavailable; please retry"))
}

func sessionCookie(value string, maxAge int) *http.Cookie {
	path := strings.TrimRight(os.Getenv("BASE_PATH"), "/") + "/"
	return &http.Cookie{Name: "pow_session", Value: value, Path: path, HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

func createAuthSession(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(data[:])
	hash := sha256.Sum256([]byte(token))
	if _, err := tx.ExecContext(ctx, "DELETE FROM auth_sessions WHERE expires_at < now()"); err != nil {
		return "", err
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO auth_sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '7 days')", hash[:], userID)
	return token, err
}

func (s *webServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "authAction")
	if action == "me" && r.Method == http.MethodGet {
		if user, ok := s.requireAccount(w, r); ok {
			writeJSON(w, http.StatusOK, map[string]any{"user": user})
		}
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	if action == "logout" {
		if cookie, err := r.Cookie("pow_session"); err == nil {
			hash := sha256.Sum256([]byte(cookie.Value))
			if _, err := s.db.ExecContext(r.Context(), "DELETE FROM auth_sessions WHERE token_hash=$1", hash[:]); err != nil {
				databaseError(w, err)
				return
			}
		}
		http.SetCookie(w, sessionCookie("", -1))
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if action != "signup" && action != "login" {
		http.NotFound(w, r)
		return
	}
	var input credentials
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	email, err := normalizeEmail(input.Email)
	if err != nil || !validPassword(input.Password) {
		writeError(w, http.StatusBadRequest, errors.New("use a valid email and a password between 12 and 72 bytes"))
		return
	}
	var attempts int
	err = s.db.QueryRowContext(r.Context(), `INSERT INTO auth_attempts(email,attempts,reset_at) VALUES($1,1,now()+interval '15 minutes') ON CONFLICT(email) DO UPDATE SET attempts=CASE WHEN auth_attempts.reset_at<now() THEN 1 ELSE auth_attempts.attempts+1 END,reset_at=CASE WHEN auth_attempts.reset_at<now() THEN now()+interval '15 minutes' ELSE auth_attempts.reset_at END RETURNING attempts`, email).Scan(&attempts)
	if err != nil {
		databaseError(w, err)
		return
	}
	if attempts > 15 {
		w.Header().Set("Retry-After", "900")
		writeError(w, http.StatusTooManyRequests, errors.New("too many attempts; try again in 15 minutes"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	var user account
	if action == "signup" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			databaseError(w, err)
			return
		}
		user, err = insertAccount(r.Context(), tx, email, hash, "user")
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, errors.New("email is already registered"))
			} else {
				databaseError(w, err)
			}
			return
		}
	} else {
		var hash []byte
		err = tx.QueryRowContext(r.Context(), "SELECT id,email,role,created_at,password_hash FROM users WHERE email=$1", email).Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt, &hash)
		if errors.Is(err, sql.ErrNoRows) {
			bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(input.Password))
			writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
			return
		}
		if err != nil {
			databaseError(w, err)
			return
		}
		if bcrypt.CompareHashAndPassword(hash, []byte(input.Password)) != nil {
			writeError(w, http.StatusUnauthorized, errors.New("invalid email or password"))
			return
		}
	}
	token, err := createAuthSession(r.Context(), tx, user.ID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "DELETE FROM auth_attempts WHERE email=$1", email)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	http.SetCookie(w, sessionCookie(token, 7*24*60*60))
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func insertAccount(ctx context.Context, tx *sql.Tx, email string, hash []byte, role string) (account, error) {
	u := account{ID: randomID(), Email: email, Role: role}
	if err := tx.QueryRowContext(ctx, "INSERT INTO users(id,email,password_hash,role) VALUES($1,$2,$3,$4) RETURNING created_at", u.ID, email, hash, role).Scan(&u.CreatedAt); err != nil {
		return u, err
	}
	p, err := profile.New(time.Now())
	if err != nil {
		return u, err
	}
	p.PlayerID = u.ID
	data, err := json.Marshal(p)
	if err != nil {
		return u, err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO player_data(user_id,profile) VALUES($1,$2)", u.ID, data)
	return u, err
}

func (s *webServer) bootstrapAdmin(ctx context.Context) error {
	email, password := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" && password == "" {
		return nil
	}
	email, err := normalizeEmail(email)
	if err != nil || !validPassword(password) {
		return errors.New("bootstrap admin requires valid email and 12–72 byte password")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(729002)"); err != nil {
		return err
	}
	var role string
	err = tx.QueryRowContext(ctx, "SELECT role FROM users WHERE email=$1", email).Scan(&role)
	if err == nil {
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := insertAccount(ctx, tx, email, hash, "admin"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *webServer) handleAdmin(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireAccount(w, r)
	if !ok {
		return
	}
	if u.Role != "admin" {
		writeError(w, http.StatusForbidden, errors.New("administrator role required"))
		return
	}
	if r.Method == http.MethodGet {
		rows, err := s.db.QueryContext(r.Context(), "SELECT id,email,role,created_at FROM users WHERE id>$1 ORDER BY id LIMIT 101", r.URL.Query().Get("after"))
		if err != nil {
			databaseError(w, err)
			return
		}
		defer rows.Close()
		users := []account{}
		for rows.Next() {
			var user account
			if err := rows.Scan(&user.ID, &user.Email, &user.Role, &user.CreatedAt); err != nil {
				databaseError(w, err)
				return
			}
			users = append(users, user)
		}
		if err := rows.Err(); err != nil {
			databaseError(w, err)
			return
		}
		next := ""
		if len(users) > 100 {
			users = users[:100]
			next = users[99].ID
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users, "next": next})
		return
	}
	if r.Method != http.MethodPatch {
		http.NotFound(w, r)
		return
	}
	id := chi.URLParam(r, "userID")
	var input struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil || (input.Role != "user" && input.Role != "admin") {
		writeError(w, http.StatusBadRequest, errors.New("role must be user or admin"))
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(729002)"); err != nil {
		databaseError(w, err)
		return
	}
	var actorRole, oldRole string
	if err := tx.QueryRowContext(r.Context(), "SELECT role FROM users WHERE id=$1", u.ID).Scan(&actorRole); err != nil {
		databaseError(w, err)
		return
	}
	if actorRole != "admin" {
		writeError(w, http.StatusForbidden, errors.New("administrator role required"))
		return
	}
	err = tx.QueryRowContext(r.Context(), "SELECT role FROM users WHERE id=$1", id).Scan(&oldRole)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if oldRole == "admin" && input.Role == "user" {
		var count int
		if err := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM users WHERE role='admin'").Scan(&count); err != nil {
			databaseError(w, err)
			return
		}
		if count == 1 {
			writeError(w, http.StatusConflict, errors.New("the last administrator cannot be demoted"))
			return
		}
	}
	_, err = tx.ExecContext(r.Context(), "UPDATE users SET role=$1 WHERE id=$2", input.Role, id)
	if err == nil && oldRole != input.Role {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO role_audit(actor_id,user_id,previous_role,new_role) VALUES($1,$2,$3,$4)", u.ID, id, oldRole, input.Role)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			expected := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")
			if expected == "" {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				expected = scheme + "://" + r.Host
			}
			parsed, err := url.Parse(expected)
			if err != nil || r.Header.Get("Sec-Fetch-Site") == "cross-site" || (origin != "" && origin != parsed.Scheme+"://"+parsed.Host) {
				writeError(w, http.StatusForbidden, errors.New("cross-origin request rejected"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
