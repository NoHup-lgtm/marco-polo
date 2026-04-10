package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

func New(logger *slog.Logger) (*DB, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	logger.Info("database initialized", "dialect", "postgres")

	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return &DB{DB: db}, nil
}

func initSchema(db *sql.DB) error {
	migrations := postgresMigrations()
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("exec migration: %w\nquery: %s", err, m)
		}
	}

	if err := ensureColumnExists(db, "users", "phone", "TEXT"); err != nil {
		return err
	}

	if err := ensureColumnExists(db, "items", "found_at", "DATE"); err != nil {
		return err
	}

	if err := seedDefaultCategories(db); err != nil {
		return err
	}

	return nil
}

func postgresMigrations() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			phone TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			category_id BIGINT,
			title TEXT NOT NULL,
			description TEXT,
			item_type TEXT NOT NULL CHECK(item_type IN ('lost', 'found')),
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'claimed', 'returned')),
			location TEXT,
			image_url TEXT,
			found_at DATE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (category_id) REFERENCES categories(id)
		)`,
		`CREATE TABLE IF NOT EXISTS claims (
			id BIGSERIAL PRIMARY KEY,
			item_id BIGINT NOT NULL,
			requester_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'accepted', 'rejected')),
			message TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (item_id) REFERENCES items(id),
			FOREIGN KEY (requester_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id BIGSERIAL PRIMARY KEY,
			token TEXT NOT NULL UNIQUE,
			user_id BIGINT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS item_returns (
			id BIGSERIAL PRIMARY KEY,
			item_id BIGINT NOT NULL UNIQUE,
			collected_by TEXT NOT NULL,
			recipient_name TEXT,
			delivered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			delivered_by_user_id BIGINT,
			FOREIGN KEY (item_id) REFERENCES items(id),
			FOREIGN KEY (delivered_by_user_id) REFERENCES users(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_type ON items(item_type)`,
		`CREATE INDEX IF NOT EXISTS idx_items_status ON items(status)`,
		`CREATE INDEX IF NOT EXISTS idx_items_category ON items(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_user ON items(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_claims_item ON claims(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_claims_requester ON claims(requester_id)`,
		`CREATE INDEX IF NOT EXISTS idx_claims_status ON claims(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,
	}
}

func ensureColumnExists(db *sql.DB, table, column, definition string) error {
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name = $1
			AND column_name = $2
		)`,
		table,
		column,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("read table info for %s: %w", table, err)
	}
	if exists {
		return nil
	}

	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}

	return nil
}

func seedDefaultCategories(db *sql.DB) error {
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
			return fmt.Errorf("seed category %s: %w", category.slug, err)
		}
	}

	return nil
}
