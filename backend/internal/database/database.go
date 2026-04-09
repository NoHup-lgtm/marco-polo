package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func New(logger *slog.Logger) (*DB, error) {
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "./marco_polo.db"
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite3: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	logger.Info("database initialized", "path", path)

	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return &DB{db}, nil
}

func initSchema(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			phone TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			category_id INTEGER,
			title TEXT NOT NULL,
			description TEXT,
			item_type TEXT NOT NULL CHECK(item_type IN ('lost', 'found')),
			status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'claimed', 'returned')),
			location TEXT,
			image_url TEXT,
			found_at DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (category_id) REFERENCES categories(id)
		)`,
		`CREATE TABLE IF NOT EXISTS claims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL,
			requester_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'accepted', 'rejected')),
			message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (item_id) REFERENCES items(id),
			FOREIGN KEY (requester_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS item_returns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL UNIQUE,
			collected_by TEXT NOT NULL,
			recipient_name TEXT,
			delivered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			delivered_by_user_id INTEGER,
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

func ensureColumnExists(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("read table info for %s: %w", table, err)
	}
	defer rows.Close()

	exists := false
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			return fmt.Errorf("scan table info for %s: %w", table, err)
		}
		if name == column {
			exists = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info for %s: %w", table, err)
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
			"INSERT OR IGNORE INTO categories (name, slug) VALUES (?, ?)",
			category.name,
			category.slug,
		); err != nil {
			return fmt.Errorf("seed category %s: %w", category.slug, err)
		}
	}

	return nil
}
