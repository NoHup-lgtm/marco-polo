package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"marco-polo/internal/models"
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

	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "username, email and password are required"})
		return
	}

	hash := hashPassword(req.Password)

	result, err := h.db.Exec("INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
		req.Username, req.Email, hash)
	if err != nil {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "username or email already exists"})
		return
	}

	id, _ := result.LastInsertId()

	token := generateToken()
	_, _ = h.db.Exec("INSERT INTO sessions (token, user_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)", token, id)

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

	hash := hashPassword(req.Password)

	var id int64
	var username string
	err := h.db.QueryRow("SELECT id, username FROM users WHERE email = ? AND password_hash = ?",
		req.Email, hash).Scan(&id, &username)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
		return
	}

	token := generateToken()
	_, _ = h.db.Exec("INSERT INTO sessions (token, user_id, created_at) VALUES (?, ?, CURRENT_TIMESTAMP)", token, id)

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

// hashPassword is a placeholder — should use bcrypt in production
func hashPassword(password string) string {
	return "$placeholder$" + password
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, resp interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
