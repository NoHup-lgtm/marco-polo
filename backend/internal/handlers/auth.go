package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"marco-polo/internal/models"

	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewAuthHandler(db *sql.DB, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{db: db, logger: logger}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "username, email and password are required"})
		return
	}

	hash, err := hashPassword(req.Password)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	result, err := h.db.Exec(
		"INSERT INTO users (username, email, password_hash, phone) VALUES (?, ?, ?, ?)",
		req.Username, req.Email, hash, strings.TrimSpace(req.Phone),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "username or email already exists"})
			return
		}

		h.logger.Error("failed to register user", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		h.logger.Error("failed to get inserted user id", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	token, err := generateToken()
	if err != nil {
		h.logger.Error("failed to generate session token", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	if _, err := h.db.Exec("INSERT INTO sessions (token, user_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)", token, id); err != nil {
		h.logger.Error("failed to persist session", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, models.Response{
		Success: true,
		Message: "user created successfully",
		Data: map[string]interface{}{
			"user_id":  id,
			"username": req.Username,
			"token":    token,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "email and password are required"})
		return
	}

	var id int64
	var username string
	var passwordHash string
	err := h.db.QueryRow("SELECT id, username, password_hash FROM users WHERE email = ?",
		req.Email).Scan(&id, &username, &passwordHash)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "invalid credentials"})
		return
	}
	if err != nil {
		h.logger.Error("failed to query user for login", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	if !verifyPassword(req.Password, passwordHash) {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "invalid credentials"})
		return
	}

	token, err := generateToken()
	if err != nil {
		h.logger.Error("failed to generate session token", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	if _, err := h.db.Exec("INSERT INTO sessions (token, user_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)", token, id); err != nil {
		h.logger.Error("failed to persist session", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{
		Success: true,
		Message: "login successful",
		Data: map[string]interface{}{
			"user_id":  id,
			"username": username,
			"token":    token,
		},
	})
}

func hashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashed), nil
}

func verifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func writeJSON(w http.ResponseWriter, status int, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
