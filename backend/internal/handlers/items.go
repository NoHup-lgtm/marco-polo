package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"marco-polo/internal/models"
)

type ItemHandler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewItemHandler(db *sql.DB, logger *slog.Logger) *ItemHandler {
	return &ItemHandler{db: db, logger: logger}
}

func (h *ItemHandler) List(w http.ResponseWriter, r *http.Request) {
	itemType := r.URL.Query().Get("type") // "lost" or "found"
	status := r.URL.Query().Get("status") // default "active"
	category := r.URL.Query().Get("category_id")

	if status == "" {
		status = "active"
	}

	query := "SELECT id, user_id, category_id, title, description, item_type, status, location, image_url, created_at, updated_at FROM items WHERE 1=1"
	args := []interface{}{}

	if itemType != "" {
		query += " AND item_type = ?"
		args = append(args, itemType)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if category != "" {
		query += " AND category_id = ?"
		args = append(args, category)
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch items"})
		return
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(&item.ID, &item.UserID, &item.CategoryID, &item.Title, &item.Description,
			&item.ItemType, &item.Status, &item.Location, &item.ImageURL, &item.CreatedAt, &item.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to read items"})
			return
		}
		items = append(items, item)
	}

	if items == nil {
		items = []models.Item{}
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Data: items})
}

func (h *ItemHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	var item models.Item
	err = h.db.QueryRow(
		"SELECT id, user_id, category_id, title, description, item_type, status, location, image_url, created_at, updated_at FROM items WHERE id = ?",
		id).Scan(&item.ID, &item.UserID, &item.CategoryID, &item.Title, &item.Description,
		&item.ItemType, &item.Status, &item.Location, &item.ImageURL, &item.CreatedAt, &item.UpdatedAt)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch item"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Data: item})
}

func (h *ItemHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "title is required"})
		return
	}
	if req.ItemType != "lost" && req.ItemType != "found" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "item_type must be 'lost' or 'found'"})
		return
	}

	userID := int64(1) // TODO: extract from JWT/middleware

	result, err := h.db.Exec(
		"INSERT INTO items (user_id, category_id, title, description, item_type, location, image_url) VALUES (?, ?, ?, ?, ?, ?, ?)",
		userID, req.CategoryID, req.Title, req.Description, req.ItemType, req.Location, req.ImageURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create item"})
		return
	}

	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, models.Response{Success: true, Message: "item created", Data: map[string]int64{"id": id}})
}

func (h *ItemHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	var req models.CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	_, err = h.db.Exec(
		"UPDATE items SET title = ?, description = ?, location = ?, category_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.Title, req.Description, req.Location, req.CategoryID, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update item"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Message: "item updated"})
}

func (h *ItemHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	_, err = h.db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to delete item"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Message: "item deleted"})
}
