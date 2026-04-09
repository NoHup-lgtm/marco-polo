# Instrucoes de execucao

## 1. Subir o backend (Go)
No terminal:

```bash
cd marco-polo/backend
DB_PATH=./marco_polo.db PORT=8080 go run ./cmd/server
```

- API: `http://localhost:8080`
- Health check: `GET http://localhost:8080/health`

## 2. Banco SQLite
O arquivo do banco fica em:

`marco-polo/backend/marco_polo.db`

Se nao existir, ele e criado automaticamente ao subir o backend (com schema + categorias padrao).

## 3. Conectar no DataGrip
1. `New` -> `Data Source` -> `SQLite`
2. Em `Database file`, selecionar:
   `marco-polo/backend/marco_polo.db`
3. Clicar em `Test Connection`
4. `Apply` / `OK`

Tabelas esperadas:
- `users`
- `sessions`
- `categories`
- `items`
- `claims`
- `item_returns`

## 4. Testar endpoints no Postman
Base URL:

`http://localhost:8080`

Fluxo recomendado:
1. `POST /api/auth/register`
2. `POST /api/auth/login`
3. Usar token em `Authorization: Bearer <token>`
4. Criar/listar itens:
   - `POST /api/items`
   - `GET /api/items`
5. Claims:
   - `POST /api/items/{id}/claims`
   - `GET /api/items/{id}/claims` (dono do item)
   - `GET /api/me/claims`
   - `PUT /api/claims/{id}`
6. Devolucao:
   - `POST /api/items/{id}/return`
7. Categorias:
   - `GET /api/categories`

## 5. Recriar banco do zero
Com o servidor parado:

```bash
cd marco-polo/backend
rm -f marco_polo.db
DB_PATH=./marco_polo.db PORT=8080 go run ./cmd/server
```
