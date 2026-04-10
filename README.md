# Marco Polo - Instrucoes para rodar o projeto

## 1. Stack e versoes
| Componente | Versao |
| --- | --- |
| Go | `1.23` |
| API backend | Go (`net/http`) |
| Banco principal containerizado | PostgreSQL `16` (Docker image `postgres:16-alpine`) |
| Driver Postgres | `github.com/lib/pq v1.10.9` |
| Seguranca senha | `golang.org/x/crypto v0.33.0` (bcrypt) |
| CI backend | `go test ./...` em `backend/` |

## 2. Pre-requisitos
1. Docker + Docker Compose plugin (`docker compose`).
2. Git.
3. DataGrip (opcional).
4. Postman/Insomnia/cURL (opcional).
5. Go 1.23 (somente para rodar sem Docker).

## 3. Rodar o projeto completo com Docker (recomendado)
Na raiz do repositorio:

```bash
docker compose up -d --build
```

Servicos:
- API: `http://localhost:8080`
- Health: `GET http://localhost:8080/health`
- PostgreSQL: `localhost:5432`

Parar stack:

```bash
docker compose down
```

Parar e remover dados do banco:

```bash
docker compose down -v
```

## 4. Configuracao de banco no Docker Compose
Credenciais padrao do `docker-compose.yml`:
- Database: `marco_polo`
- User: `marco_polo`
- Password: `marco_polo`
- Host interno da API: `postgres`
- Host para ferramentas locais (DataGrip): `localhost`
- Porta: `5432`

`DATABASE_URL` usado pela API:

`postgres://marco_polo:marco_polo@postgres:5432/marco_polo?sslmode=disable`

## 5. Conectar no DataGrip (PostgreSQL em Docker)
1. `New` -> `Data Source` -> `PostgreSQL`.
2. Preencha:
   - Host: `localhost`
   - Port: `5432`
   - Database: `marco_polo`
   - User: `marco_polo`
   - Password: `marco_polo`
3. Clique em `Test Connection`.
4. `Apply` / `OK`.

Tabelas esperadas:
- `users`
- `sessions`
- `categories`
- `items`
- `claims`
- `item_returns`

## 6. Rodar sem Docker (modo local alternativo)
### Com PostgreSQL local
```bash
cd backend
DATABASE_URL="postgres://USER:PASSWORD@localhost:5432/marco_polo?sslmode=disable" PORT=8080 go run ./cmd/server
```

## 7. Rodar testes do backend
```bash
cd backend
go test ./...
```

## 8. Endpoints principais (Postman)
Base URL: `http://localhost:8080`

### Auth
- `POST /api/auth/register`
- `POST /api/auth/login`

### Categorias
- `GET /api/categories`

### Itens
- `GET /api/items`
  - filtros: `type`, `status`, `category_id`, `q`, `location`, `found_from`, `found_to`
- `GET /api/items/{id}`
- `POST /api/items` (Bearer token)
- `PUT /api/items/{id}` (Bearer token, dono)
- `DELETE /api/items/{id}` (Bearer token, dono)

### Claims
- `POST /api/items/{id}/claims` (Bearer token)
- `GET /api/items/{id}/claims` (Bearer token, dono do item)
- `GET /api/me/claims` (Bearer token)
- `PUT /api/claims/{id}` (Bearer token, dono do item)

### Devolucao
- `POST /api/items/{id}/return` (Bearer token, dono do item)

Header protegido:

`Authorization: Bearer <token>`

## 9. Troubleshooting
- **`docker: command not found`**: instale Docker Desktop/Engine e habilite `docker compose`.
- **Porta 8080 ocupada**: altere mapeamento da API no `docker-compose.yml`.
- **Porta 5432 ocupada**: altere mapeamento do Postgres no `docker-compose.yml`.
- **Erro de conexao API -> Postgres**: confirme se o servico `postgres` esta `healthy`.
- **Reset completo do banco Docker**: use `docker compose down -v` e suba novamente.
