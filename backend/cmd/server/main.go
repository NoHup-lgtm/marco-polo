package main

import (
	"log/slog"
	"net/http"
	"os"

	"marco-polo/internal/database"
	"marco-polo/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := database.New(logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	router := server.NewRouter(db.DB, logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("server starting", "port", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
