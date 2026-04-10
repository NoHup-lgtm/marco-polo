package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"marco-polo/internal/database"
)

type apiResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func TestHealthEndpoint(t *testing.T) {
	router := setupRouter(t)

	res := performRequest(t, router, http.MethodGet, "/health", nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	if body := strings.TrimSpace(res.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRegisterAndLoginEndpoints(t *testing.T) {
	router := setupRouter(t)

	token := registerUser(t, router, "auth")
	if token == "" {
		t.Fatal("expected non-empty token from register")
	}

	loginReq := map[string]string{
		"email":    "auth@example.com",
		"password": "123456",
	}

	res := performRequest(t, router, http.MethodPost, "/api/auth/login", loginReq, "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, res.Code, res.Body.String())
	}

	var resp apiResponse
	decodeJSON(t, res.Body.Bytes(), &resp)
	if !resp.Success {
		t.Fatalf("expected success=true, got false: %s", resp.Error)
	}

	var data struct {
		Token string `json:"token"`
	}
	decodeJSON(t, resp.Data, &data)
	if data.Token == "" {
		t.Fatal("expected non-empty token from login")
	}
}

func TestCreateItemRequiresAuth(t *testing.T) {
	router := setupRouter(t)

	createReq := map[string]any{
		"title":       "Carteira",
		"description": "Carteira preta encontrada",
		"item_type":   "found",
	}

	res := performRequest(t, router, http.MethodPost, "/api/items", createReq, "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestItemEndpointsFlow(t *testing.T) {
	router := setupRouter(t)
	token := registerUser(t, router, "items")

	createReq := map[string]any{
		"title":       "Mochila",
		"description": "Mochila azul",
		"item_type":   "found",
		"location":    "Biblioteca",
		"found_at":    "2026-04-08",
	}

	createRes := performRequest(t, router, http.MethodPost, "/api/items", createReq, token)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusCreated, createRes.Code, createRes.Body.String())
	}

	var createResp apiResponse
	decodeJSON(t, createRes.Body.Bytes(), &createResp)
	var createData struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp.Data, &createData)
	if createData.ID == 0 {
		t.Fatal("expected non-zero item id")
	}

	listRes := performRequest(t, router, http.MethodGet, "/api/items", nil, "")
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listRes.Code)
	}

	var listResp apiResponse
	decodeJSON(t, listRes.Body.Bytes(), &listResp)
	var items []map[string]any
	decodeJSON(t, listResp.Data, &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 item in list, got %d", len(items))
	}

	getRes := performRequest(t, router, http.MethodGet, "/api/items/"+strconv.FormatInt(createData.ID, 10), nil, "")
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, getRes.Code)
	}

	updateReq := map[string]any{
		"title":       "Mochila Atualizada",
		"description": "Mochila azul com etiqueta",
		"item_type":   "found",
		"location":    "Secretaria",
		"found_at":    "2026-04-09",
	}
	updateRes := performRequest(t, router, http.MethodPut, "/api/items/"+strconv.FormatInt(createData.ID, 10), updateReq, token)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, updateRes.Code, updateRes.Body.String())
	}

	deleteRes := performRequest(t, router, http.MethodDelete, "/api/items/"+strconv.FormatInt(createData.ID, 10), nil, token)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deleteRes.Code)
	}

	getAfterDeleteRes := performRequest(t, router, http.MethodGet, "/api/items/"+strconv.FormatInt(createData.ID, 10), nil, "")
	if getAfterDeleteRes.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, getAfterDeleteRes.Code)
	}
}

func TestItemOwnershipEnforced(t *testing.T) {
	router := setupRouter(t)
	ownerToken := registerUser(t, router, "owner")
	otherToken := registerUser(t, router, "other")

	createReq := map[string]any{
		"title":       "Caderno",
		"description": "Caderno vermelho",
		"item_type":   "found",
	}

	createRes := performRequest(t, router, http.MethodPost, "/api/items", createReq, ownerToken)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, createRes.Code)
	}

	var createResp apiResponse
	decodeJSON(t, createRes.Body.Bytes(), &createResp)
	var createData struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp.Data, &createData)

	updateReq := map[string]any{
		"title":       "Caderno Tentativa",
		"description": "Tentativa de edição",
		"item_type":   "found",
	}
	updateRes := performRequest(t, router, http.MethodPut, "/api/items/"+strconv.FormatInt(createData.ID, 10), updateReq, otherToken)
	if updateRes.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, updateRes.Code)
	}

	deleteRes := performRequest(t, router, http.MethodDelete, "/api/items/"+strconv.FormatInt(createData.ID, 10), nil, otherToken)
	if deleteRes.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, deleteRes.Code)
	}
}

func TestCategoriesEndpoint(t *testing.T) {
	router := setupRouter(t)

	res := performRequest(t, router, http.MethodGet, "/api/categories", nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, res.Code, res.Body.String())
	}

	var resp apiResponse
	decodeJSON(t, res.Body.Bytes(), &resp)
	if !resp.Success {
		t.Fatalf("expected success=true, got false: %s", resp.Error)
	}

	var categories []map[string]any
	decodeJSON(t, resp.Data, &categories)
	if len(categories) == 0 {
		t.Fatal("expected seeded categories")
	}
}

func TestClaimsAndReturnFlow(t *testing.T) {
	router := setupRouter(t)
	ownerToken := registerUser(t, router, "owner_claims")
	requesterToken := registerUser(t, router, "requester_claims")
	otherToken := registerUser(t, router, "other_claims")

	itemID := createItem(t, router, ownerToken, map[string]any{
		"title":       "Notebook Dell",
		"description": "Notebook prata",
		"item_type":   "lost",
		"location":    "Laboratorio 2",
		"found_at":    "2026-04-10",
	})

	claimRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", map[string]any{
		"message": "Acho que eh meu, posso comprovar com adesivos.",
	}, requesterToken)
	if claimRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusCreated, claimRes.Code, claimRes.Body.String())
	}

	duplicateClaimRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", map[string]any{
		"message": "segunda tentativa",
	}, requesterToken)
	if duplicateClaimRes.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, duplicateClaimRes.Code)
	}

	ownerClaimRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", map[string]any{
		"message": "sou o dono",
	}, ownerToken)
	if ownerClaimRes.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, ownerClaimRes.Code)
	}

	listByOtherRes := performRequest(t, router, http.MethodGet, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", nil, otherToken)
	if listByOtherRes.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, listByOtherRes.Code)
	}

	listByOwnerRes := performRequest(t, router, http.MethodGet, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", nil, ownerToken)
	if listByOwnerRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listByOwnerRes.Code)
	}

	var listByOwnerResp apiResponse
	decodeJSON(t, listByOwnerRes.Body.Bytes(), &listByOwnerResp)
	var claims []map[string]any
	decodeJSON(t, listByOwnerResp.Data, &claims)
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}

	claimID := int64(claims[0]["id"].(float64))
	updateClaimRes := performRequest(t, router, http.MethodPut, "/api/claims/"+strconv.FormatInt(claimID, 10), map[string]any{
		"status": "accepted",
	}, ownerToken)
	if updateClaimRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, updateClaimRes.Code, updateClaimRes.Body.String())
	}

	secondRequesterToken := registerUser(t, router, "requester_claims_2")
	claimAfterAcceptedRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/claims", map[string]any{
		"message": "quero tentar",
	}, secondRequesterToken)
	if claimAfterAcceptedRes.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, claimAfterAcceptedRes.Code)
	}

	myClaimsRes := performRequest(t, router, http.MethodGet, "/api/me/claims", nil, requesterToken)
	if myClaimsRes.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, myClaimsRes.Code)
	}

	var myClaimsResp apiResponse
	decodeJSON(t, myClaimsRes.Body.Bytes(), &myClaimsResp)
	var myClaims []map[string]any
	decodeJSON(t, myClaimsResp.Data, &myClaims)
	if len(myClaims) == 0 {
		t.Fatal("expected requester claims to be listed")
	}

	returnByOtherRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/return", map[string]any{
		"collected_by":   "Secretaria",
		"recipient_name": "Aluno X",
	}, otherToken)
	if returnByOtherRes.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, returnByOtherRes.Code)
	}

	returnRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/return", map[string]any{
		"collected_by":   "Secretaria",
		"recipient_name": "Aluno X",
	}, ownerToken)
	if returnRes.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusCreated, returnRes.Code, returnRes.Body.String())
	}

	duplicateReturnRes := performRequest(t, router, http.MethodPost, "/api/items/"+strconv.FormatInt(itemID, 10)+"/return", map[string]any{
		"collected_by": "Secretaria",
	}, ownerToken)
	if duplicateReturnRes.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, duplicateReturnRes.Code)
	}
}

func TestItemsListFilters(t *testing.T) {
	router := setupRouter(t)
	token := registerUser(t, router, "filters")

	createItem(t, router, token, map[string]any{
		"title":       "Chave do carro",
		"description": "chave com chaveiro azul",
		"item_type":   "found",
		"location":    "Portaria",
		"found_at":    "2026-04-08",
	})
	createItem(t, router, token, map[string]any{
		"title":       "Calculadora",
		"description": "calculadora cientifica",
		"item_type":   "found",
		"location":    "Sala 10",
		"found_at":    "2026-04-11",
	})

	res := performRequest(t, router, http.MethodGet, "/api/items?q=chave&location=Portaria&found_from=2026-04-01&found_to=2026-04-09", nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusOK, res.Code, res.Body.String())
	}

	var resp apiResponse
	decodeJSON(t, res.Body.Bytes(), &resp)
	var items []map[string]any
	decodeJSON(t, resp.Data, &items)
	if len(items) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(items))
	}

	invalidDateRes := performRequest(t, router, http.MethodGet, "/api/items?found_from=2026-99-99", nil, "")
	if invalidDateRes.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, invalidDateRes.Code)
	}
}

func setupRouter(t *testing.T) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://marco_polo:marco_polo@localhost:5432/marco_polo?sslmode=disable"
	}
	t.Setenv("DATABASE_URL", databaseURL)

	db, err := database.New(logger)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	resetTestDatabase(t, db.DB)

	return NewRouter(db.DB, logger)
}

func resetTestDatabase(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec("TRUNCATE TABLE item_returns, claims, items, sessions, users, categories RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("failed to truncate test tables: %v", err)
	}

	defaultCategories := []struct {
		name string
		slug string
	}{
		{name: "Documentos", slug: "documentos"},
		{name: "Eletronicos", slug: "eletronicos"},
		{name: "Roupas", slug: "roupas"},
		{name: "Acessorios", slug: "acessorios"},
		{name: "Material Escolar", slug: "material-escolar"},
		{name: "Outros", slug: "outros"},
	}

	for _, category := range defaultCategories {
		if _, err := db.Exec(
			"INSERT INTO categories (name, slug) VALUES ($1, $2) ON CONFLICT (slug) DO NOTHING",
			category.name,
			category.slug,
		); err != nil {
			t.Fatalf("failed to seed category %s: %v", category.slug, err)
		}
	}
}

func registerUser(t *testing.T, router http.Handler, suffix string) string {
	t.Helper()

	req := map[string]string{
		"username": "user_" + suffix,
		"email":    suffix + "@example.com",
		"password": "123456",
		"phone":    "11999999999",
	}
	res := performRequest(t, router, http.MethodPost, "/api/auth/register", req, "")
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusCreated, res.Code, res.Body.String())
	}

	var resp apiResponse
	decodeJSON(t, res.Body.Bytes(), &resp)
	if !resp.Success {
		t.Fatalf("register failed: %s", resp.Error)
	}

	var data struct {
		Token string `json:"token"`
	}
	decodeJSON(t, resp.Data, &data)
	if data.Token == "" {
		t.Fatal("expected register token")
	}

	return data.Token
}

func createItem(t *testing.T, router http.Handler, token string, req map[string]any) int64 {
	t.Helper()

	res := performRequest(t, router, http.MethodPost, "/api/items", req, token)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d (body=%s)", http.StatusCreated, res.Code, res.Body.String())
	}

	var createResp apiResponse
	decodeJSON(t, res.Body.Bytes(), &createResp)
	var data struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, createResp.Data, &data)
	if data.ID == 0 {
		t.Fatal("expected non-zero item id")
	}
	return data.ID
}

func performRequest(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader io.Reader = http.NoBody
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func decodeJSON(t *testing.T, payload []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(payload, out); err != nil {
		t.Fatalf("failed to decode json: %v (payload=%s)", err, string(payload))
	}
}
