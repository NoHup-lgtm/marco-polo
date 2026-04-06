package main

import (
	"log/slog"
	"net/http"
	"os"

	"marco-polo/internal/database"
	"marco-polo/internal/handlers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := database.New(logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	router := http.NewServeMux()

	authHandler := handlers.NewAuthHandler(db.DB, logger)
	itemHandler := handlers.NewItemHandler(db.DB, logger)

	// Health check
	router.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth routes
	router.HandleFunc("POST /api/auth/register", authHandler.Register)
	router.HandleFunc("POST /api/auth/login", authHandler.Login)

	// Item routes
	router.HandleFunc("GET /api/items", itemHandler.List)
	router.HandleFunc("GET /api/items/{id}", itemHandler.GetByID)
	router.HandleFunc("POST /api/items", itemHandler.Create)
	router.HandleFunc("PUT /api/items/{id}", itemHandler.Update)
	router.HandleFunc("DELETE /api/items/{id}", itemHandler.Delete)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("server starting", "port", port)
	if err := http.ListenAndServe(":"+port, loggerMiddleware(router, logger)); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func loggerMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
