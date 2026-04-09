package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"marco-polo/internal/models"
)

type contextKey string

const userIDContextKey contextKey = "authenticated_user_id"

type AuthMiddleware struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewAuthMiddleware(db *sql.DB, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{db: db, logger: logger}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing or invalid authorization token"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing or invalid authorization token"})
			return
		}

		var userID int64
		err := m.db.QueryRow("SELECT user_id FROM sessions WHERE token = ?", token).Scan(&userID)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "invalid session token"})
			return
		}
		if err != nil {
			m.logger.Error("failed to validate session token", "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "internal server error"})
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDContextKey).(int64)
	return userID, ok
}
