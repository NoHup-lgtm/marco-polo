package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"marco-polo/internal/models"
)

func (h *ItemHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	itemID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	requesterID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	var req models.CreateClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	var ownerID int64
	var itemStatus string
	err = dbQueryRow(h.db, "SELECT user_id, status FROM items WHERE id = ?", itemID).Scan(&ownerID, &itemStatus)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to query item for claim", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create claim"})
		return
	}

	if ownerID == requesterID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "item owner cannot claim own item"})
		return
	}
	if itemStatus != "active" {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "item is not available for claims"})
		return
	}

	var existingClaimID int64
	err = dbQueryRow(
		h.db,
		"SELECT id FROM claims WHERE item_id = ? AND requester_id = ? AND status = 'pending'",
		itemID,
		requesterID,
	).Scan(&existingClaimID)
	if err == nil {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "pending claim already exists for this item"})
		return
	}
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("failed to check duplicate claim", "item_id", itemID, "requester_id", requesterID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create claim"})
		return
	}

	claimID, err := insertAndReturnID(
		h.db,
		"INSERT INTO claims (item_id, requester_id, status, message, created_at) VALUES (?, ?, 'pending', ?, CURRENT_TIMESTAMP)",
		itemID,
		requesterID,
		strings.TrimSpace(req.Message),
	)
	if err != nil {
		h.logger.Error("failed to insert claim", "item_id", itemID, "requester_id", requesterID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to create claim"})
		return
	}

	writeJSON(w, http.StatusCreated, models.Response{
		Success: true,
		Message: "claim created",
		Data: map[string]interface{}{
			"id":      claimID,
			"item_id": itemID,
			"status":  "pending",
		},
	})
}

func (h *ItemHandler) ListItemClaims(w http.ResponseWriter, r *http.Request) {
	itemID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	authUserID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	var ownerID int64
	err = dbQueryRow(h.db, "SELECT user_id FROM items WHERE id = ?", itemID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to query item owner", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
		return
	}

	if ownerID != authUserID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "only item owner can list claims"})
		return
	}

	rows, err := dbQuery(
		h.db,
		`SELECT c.id, c.item_id, c.requester_id, c.status, c.message, c.created_at, u.username
		 FROM claims c
		 JOIN users u ON u.id = c.requester_id
		 WHERE c.item_id = ?
		 ORDER BY c.created_at DESC`,
		itemID,
	)
	if err != nil {
		h.logger.Error("failed to query claims by item", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
		return
	}
	defer rows.Close()

	type claimView struct {
		ID          int64  `json:"id"`
		ItemID      int64  `json:"item_id"`
		RequesterID int64  `json:"requester_id"`
		Requester   string `json:"requester"`
		Status      string `json:"status"`
		Message     string `json:"message,omitempty"`
		CreatedAt   string `json:"created_at"`
	}

	claims := make([]claimView, 0)
	for rows.Next() {
		var (
			claim     claimView
			message   sql.NullString
			createdAt time.Time
		)
		if err := rows.Scan(
			&claim.ID,
			&claim.ItemID,
			&claim.RequesterID,
			&claim.Status,
			&message,
			&createdAt,
			&claim.Requester,
		); err != nil {
			h.logger.Error("failed to scan claim row", "item_id", itemID, "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
			return
		}
		if message.Valid {
			claim.Message = message.String
		}
		claim.CreatedAt = createdAt.Format(time.RFC3339)
		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		h.logger.Error("failed while iterating claim rows", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Data: claims})
}

func (h *ItemHandler) ListMyClaims(w http.ResponseWriter, r *http.Request) {
	requesterID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	rows, err := dbQuery(
		h.db,
		`SELECT c.id, c.item_id, c.status, c.message, c.created_at, i.title
		 FROM claims c
		 JOIN items i ON i.id = c.item_id
		 WHERE c.requester_id = ?
		 ORDER BY c.created_at DESC`,
		requesterID,
	)
	if err != nil {
		h.logger.Error("failed to query user claims", "requester_id", requesterID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
		return
	}
	defer rows.Close()

	type myClaimView struct {
		ID        int64  `json:"id"`
		ItemID    int64  `json:"item_id"`
		ItemTitle string `json:"item_title"`
		Status    string `json:"status"`
		Message   string `json:"message,omitempty"`
		CreatedAt string `json:"created_at"`
	}

	claims := make([]myClaimView, 0)
	for rows.Next() {
		var (
			claim     myClaimView
			message   sql.NullString
			createdAt time.Time
		)
		if err := rows.Scan(&claim.ID, &claim.ItemID, &claim.Status, &message, &createdAt, &claim.ItemTitle); err != nil {
			h.logger.Error("failed to scan user claim row", "requester_id", requesterID, "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
			return
		}
		if message.Valid {
			claim.Message = message.String
		}
		claim.CreatedAt = createdAt.Format(time.RFC3339)
		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		h.logger.Error("failed while iterating user claim rows", "requester_id", requesterID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to fetch claims"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{Success: true, Data: claims})
}

func (h *ItemHandler) UpdateClaimStatus(w http.ResponseWriter, r *http.Request) {
	claimID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid claim id"})
		return
	}

	authUserID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	var req models.UpdateClaimStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	req.Status = strings.TrimSpace(strings.ToLower(req.Status))
	if req.Status != "accepted" && req.Status != "rejected" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "status must be 'accepted' or 'rejected'"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		h.logger.Error("failed to start claim status transaction", "claim_id", claimID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
		return
	}
	defer tx.Rollback()

	var (
		itemID      int64
		current     string
		itemOwnerID int64
		itemStatus  string
	)
	err = txQueryRow(
		tx,
		`SELECT c.item_id, c.status, i.user_id, i.status
		 FROM claims c
		 JOIN items i ON i.id = c.item_id
		 WHERE c.id = ?`,
		claimID,
	).Scan(&itemID, &current, &itemOwnerID, &itemStatus)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "claim not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to load claim", "claim_id", claimID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
		return
	}

	if itemOwnerID != authUserID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "only item owner can update claim status"})
		return
	}
	if current != "pending" {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "claim is not pending"})
		return
	}

	if req.Status == "accepted" && itemStatus != "active" {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "item is not available for claim acceptance"})
		return
	}

	if _, err := txExec(tx, "UPDATE claims SET status = ? WHERE id = ?", req.Status, claimID); err != nil {
		h.logger.Error("failed to update claim status", "claim_id", claimID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
		return
	}

	if req.Status == "accepted" {
		if _, err := txExec(tx, "UPDATE items SET status = 'claimed', updated_at = CURRENT_TIMESTAMP WHERE id = ?", itemID); err != nil {
			h.logger.Error("failed to update item status to claimed", "item_id", itemID, "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
			return
		}
		if _, err := txExec(tx, "UPDATE claims SET status = 'rejected' WHERE item_id = ? AND id <> ? AND status = 'pending'", itemID, claimID); err != nil {
			h.logger.Error("failed to reject competing claims", "item_id", itemID, "error", err)
			writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.logger.Error("failed to commit claim status transaction", "claim_id", claimID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to update claim"})
		return
	}

	writeJSON(w, http.StatusOK, models.Response{
		Success: true,
		Message: "claim status updated",
		Data: map[string]interface{}{
			"id":      claimID,
			"item_id": itemID,
			"status":  req.Status,
		},
	})
}

func (h *ItemHandler) RegisterReturn(w http.ResponseWriter, r *http.Request) {
	itemID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid item id"})
		return
	}

	authUserID, ok := userIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, models.Response{Success: false, Error: "missing authenticated user"})
		return
	}

	var req models.RegisterItemReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "invalid request body"})
		return
	}

	req.CollectedBy = strings.TrimSpace(req.CollectedBy)
	req.RecipientName = strings.TrimSpace(req.RecipientName)
	if req.CollectedBy == "" {
		writeJSON(w, http.StatusBadRequest, models.Response{Success: false, Error: "collected_by is required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		h.logger.Error("failed to start return transaction", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to register return"})
		return
	}
	defer tx.Rollback()

	var ownerID int64
	var itemStatus string
	err = txQueryRow(tx, "SELECT user_id, status FROM items WHERE id = ?", itemID).Scan(&ownerID, &itemStatus)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, models.Response{Success: false, Error: "item not found"})
		return
	}
	if err != nil {
		h.logger.Error("failed to load item for return", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to register return"})
		return
	}

	if ownerID != authUserID {
		writeJSON(w, http.StatusForbidden, models.Response{Success: false, Error: "only item owner can register return"})
		return
	}
	if itemStatus != "claimed" {
		writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "item must be claimed before return"})
		return
	}

	if _, err := txExec(
		tx,
		"INSERT INTO item_returns (item_id, collected_by, recipient_name, delivered_by_user_id, delivered_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)",
		itemID,
		req.CollectedBy,
		req.RecipientName,
		authUserID,
	); err != nil {
		if isUniqueConstraintError(err) {
			writeJSON(w, http.StatusConflict, models.Response{Success: false, Error: "return already registered for this item"})
			return
		}
		h.logger.Error("failed to insert item return", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to register return"})
		return
	}

	if _, err := txExec(tx, "UPDATE items SET status = 'returned', updated_at = CURRENT_TIMESTAMP WHERE id = ?", itemID); err != nil {
		h.logger.Error("failed to update item to returned", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to register return"})
		return
	}

	if err := tx.Commit(); err != nil {
		h.logger.Error("failed to commit return transaction", "item_id", itemID, "error", err)
		writeJSON(w, http.StatusInternalServerError, models.Response{Success: false, Error: "failed to register return"})
		return
	}

	writeJSON(w, http.StatusCreated, models.Response{
		Success: true,
		Message: "item return registered",
		Data: map[string]interface{}{
			"item_id": itemID,
			"status":  "returned",
		},
	})
}
