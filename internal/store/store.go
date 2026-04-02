package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct { db *sql.DB }

type Object struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Bucket       string   `json:"bucket"`
	ContentType  string   `json:"content_type"`
	Size         int      `json:"size"`
	Path         string   `json:"path"`
	CreatedAt    string   `json:"created_at"`
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	dsn := filepath.Join(dataDir, "granary.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS objects (
			id TEXT PRIMARY KEY,\n\t\t\tname TEXT DEFAULT '',\n\t\t\tbucket TEXT DEFAULT 'default',\n\t\t\tcontent_type TEXT DEFAULT '',\n\t\t\tsize INTEGER DEFAULT 0,\n\t\t\tpath TEXT DEFAULT '',
			created_at TEXT DEFAULT (datetime('now'))
		)`)
	if err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error { return d.db.Close() }

func genID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func (d *DB) Create(e *Object) error {
	e.ID = genID()
	e.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`INSERT INTO objects (id, name, bucket, content_type, size, path, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Name, e.Bucket, e.ContentType, e.Size, e.Path, e.CreatedAt)
	return err
}

func (d *DB) Get(id string) *Object {
	row := d.db.QueryRow(`SELECT id, name, bucket, content_type, size, path, created_at FROM objects WHERE id=?`, id)
	var e Object
	if err := row.Scan(&e.ID, &e.Name, &e.Bucket, &e.ContentType, &e.Size, &e.Path, &e.CreatedAt); err != nil {
		return nil
	}
	return &e
}

func (d *DB) List() []Object {
	rows, err := d.db.Query(`SELECT id, name, bucket, content_type, size, path, created_at FROM objects ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []Object
	for rows.Next() {
		var e Object
		if err := rows.Scan(&e.ID, &e.Name, &e.Bucket, &e.ContentType, &e.Size, &e.Path, &e.CreatedAt); err != nil {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (d *DB) Delete(id string) error {
	_, err := d.db.Exec(`DELETE FROM objects WHERE id=?`, id)
	return err
}

func (d *DB) Count() int {
	var n int
	d.db.QueryRow(`SELECT COUNT(*) FROM objects`).Scan(&n)
	return n
}
