package handlers

import (
	"database/sql"
	"log/slog"
	"net/http"

	"marco-polo/internal/models"
)

type CategoryHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewCategoryHandler(db *sql.DB, logger *slog.Logger) *CategoryHandler {
	return &CategoryHandler{db: db, logger: logger}
}

func (h *CategoryHandler) List(w http.ResponseWriter, _ *http.Request) {
	rows, err := dbQuery(h.db, "SELECT id, name, slug FROM categories ORDER BY name ASC")
	if err != nil {
		h.logger.Error("failed to query categories", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch categories"})
		return
	}
	defer rows.Close()

	categories := make([]models.Category, 0)
	for rows.Next() {
		var category models.Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Slug); err != nil {
			h.logger.Error("failed to scan categories row", "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch categories"})
			return
		}
		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		h.logger.Error("failed while iterating categories rows", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch categories"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Data: categories})
}
