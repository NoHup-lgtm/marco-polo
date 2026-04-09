package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	location := strings.TrimSpace(r.URL.Query().Get("location"))
	foundFrom := strings.TrimSpace(r.URL.Query().Get("found_from"))
	foundTo := strings.TrimSpace(r.URL.Query().Get("found_to"))

	if status == "" {
		status = "active"
	}

	query := "SELECT id, user_id, category_id, title, description, item_type, status, location, image_url, found_at, created_at, updated_at FROM items WHERE 1=1"
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
		categoryID, err := strconv.ParseInt(category, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid category_id"})
			return
		}
		query += " AND category_id = ?"
		args = append(args, categoryID)
	}
	if search != "" {
		searchLike := "%" + search + "%"
		query += " AND (title LIKE ? OR description LIKE ? OR location LIKE ?)"
		args = append(args, searchLike, searchLike, searchLike)
	}
	if location != "" {
		query += " AND location LIKE ?"
		args = append(args, "%"+location+"%")
	}
	if foundFrom != "" {
		if _, err := time.Parse("2006-01-02", foundFrom); err != nil {
			writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "found_from must be YYYY-MM-DD"})
			return
		}
		query += " AND found_at >= ?"
		args = append(args, foundFrom)
	}
	if foundTo != "" {
		if _, err := time.Parse("2006-01-02", foundTo); err != nil {
			writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "found_to must be YYYY-MM-DD"})
			return
		}
		query += " AND found_at <= ?"
		args = append(args, foundTo)
	}

	query += " ORDER BY created_at DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.logger.Error("failed to query items list", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch items"})
		return
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var item models.Item
		var foundAt sql.NullString
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.CategoryID, &item.Title, &item.Description,
			&item.ItemType, &item.Status, &item.Location, &item.ImageURL, &foundAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			h.logger.Error("failed to scan item row", "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to read items"})
			return
		}
		parsedFoundAt, err := parseStoredDate(foundAt)
		if err != nil {
			h.logger.Error("failed to parse found_at", "raw", foundAt.String, "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to read items"})
			return
		}
		item.FoundAt = parsedFoundAt
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
	var foundAt sql.NullString
	err = h.db.QueryRow(
		"SELECT id, user_id, category_id, title, description, item_type, status, location, image_url, found_at, created_at, updated_at FROM items WHERE id = ?",
		id,
	).Scan(
		&item.ID, &item.UserID, &item.CategoryID, &item.Title, &item.Description,
		&item.ItemType, &item.Status, &item.Location, &item.ImageURL, &foundAt, &item.CreatedAt, &item.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to fetch item by id", "item_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch item"})
		return
	}

	parsedFoundAt, err := parseStoredDate(foundAt)
	if err != nil {
		h.logger.Error("failed to parse found_at", "raw", foundAt.String, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch item"})
		return
	}
	item.FoundAt = parsedFoundAt

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
	if req.ItemType != "lost" && req.ItemType != "found" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "item_type must be 'lost' or 'found'"})
		return
	}

	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	foundAt, err := normalizeFoundAt(req.FoundAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "found_at must be YYYY-MM-DD"})
		return
	}

	result, err := h.db.Exec(
		"INSERT INTO items (user_id, category_id, title, description, item_type, location, image_url, found_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		userID, req.CategoryID, req.Title, req.Description, req.ItemType, req.Location, req.ImageURL, foundAt,
	)
	if err != nil {
		h.logger.Error("failed to create item", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create item"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		h.logger.Error("failed to read inserted item id", "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create item"})
		return
	}

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

	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "title is required"})
		return
	}

	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	ownerID, exists, err := h.findItemOwner(id)
	if err != nil {
		h.logger.Error("failed to check item owner", "item_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update item"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if ownerID != userID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "item does not belong to authenticated user"})
		return
	}

	foundAt, err := normalizeFoundAt(req.FoundAt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "found_at must be YYYY-MM-DD"})
		return
	}

	_, err = h.db.Exec(
		"UPDATE items SET title = ?, description = ?, location = ?, category_id = ?, found_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.Title, req.Description, req.Location, req.CategoryID, foundAt, id,
	)
	if err != nil {
		h.logger.Error("failed to update item", "item_id", id, "error", err)
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

	userID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	ownerID, exists, err := h.findItemOwner(id)
	if err != nil {
		h.logger.Error("failed to check item owner", "item_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to delete item"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if ownerID != userID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "item does not belong to authenticated user"})
		return
	}

	_, err = h.db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		h.logger.Error("failed to delete item", "item_id", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to delete item"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Message: "item deleted"})
}

func (h *ItemHandler) findItemOwner(itemID int64) (int64, bool, error) {
	var ownerID int64
	err := h.db.QueryRow("SELECT user_id FROM items WHERE id = ?", itemID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return ownerID, true, nil
}

func normalizeFoundAt(foundAt string) (interface{}, error) {
	trimmed := strings.TrimSpace(foundAt)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, err
	}

	return parsed.Format("2006-01-02"), nil
}

func parseStoredDate(foundAt sql.NullString) (*time.Time, error) {
	if !foundAt.Valid || strings.TrimSpace(foundAt.String) == "" {
		return nil, nil
	}

	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		time.RFC3339,
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, foundAt.String); err == nil {
			return &parsed, nil
		}
	}

	return nil, strconv.ErrSyntax
}
