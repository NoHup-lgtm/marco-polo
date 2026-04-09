package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"marco-polo/internal/handlers"
)

func NewRouter(db *sql.DB, logger *slog.Logger) http.Handler {
	router := http.NewServeMux()

	authHandler := handlers.NewAuthHandler(db, logger)
	itemHandler := handlers.NewItemHandler(db, logger)
	categoryHandler := handlers.NewCategoryHandler(db, logger)
	authMiddleware := handlers.NewAuthMiddleware(db, logger)

	// Health check
	router.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes
	router.HandleFunc("POST /api/auth/register", authHandler.Register)
	router.HandleFunc("POST /api/auth/login", authHandler.Login)

	// Public item routes
	router.HandleFunc("GET /api/items", itemHandler.List)
	router.HandleFunc("GET /api/items/{id}", itemHandler.GetByID)
	router.HandleFunc("GET /api/categories", categoryHandler.List)

	// Protected item routes
	router.Handle("POST /api/items", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.Create)))
	router.Handle("PUT /api/items/{id}", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.Update)))
	router.Handle("DELETE /api/items/{id}", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.Delete)))
	router.Handle("POST /api/items/{id}/claims", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.CreateClaim)))
	router.Handle("GET /api/items/{id}/claims", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.ListItemClaims)))
	router.Handle("POST /api/items/{id}/return", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.RegisterReturn)))
	router.Handle("GET /api/me/claims", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.ListMyClaims)))
	router.Handle("PUT /api/claims/{id}", authMiddleware.RequireAuth(http.HandlerFunc(itemHandler.UpdateClaimStatus)))

	return loggerMiddleware(router, logger)
}

func loggerMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
