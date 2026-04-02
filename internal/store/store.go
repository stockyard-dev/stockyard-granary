package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	db      *sql.DB
	dataDir string
}

type Bucket struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Public      bool   `json:"public"`
	CreatedAt   string `json:"created_at"`
	ObjectCount int    `json:"object_count"`
	TotalSize   int64  `json:"total_size"`
}

type Object struct {
	ID          string `json:"id"`
	BucketID    string `json:"bucket_id"`
	BucketName  string `json:"bucket_name,omitempty"`
	Key         string `json:"key"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "files"), 0755); err != nil {
		return nil, err
	}
	dsn := filepath.Join(dataDir, "granary.db") + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS buckets (id TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL, public INTEGER DEFAULT 0, created_at TEXT DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS objects (id TEXT PRIMARY KEY, bucket_id TEXT NOT NULL REFERENCES buckets(id), key TEXT NOT NULL, content_type TEXT DEFAULT 'application/octet-stream', size INTEGER DEFAULT 0, hash TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')), UNIQUE(bucket_id, key))`,
		`CREATE INDEX IF NOT EXISTS idx_objects_bucket ON objects(bucket_id)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	return &DB{db: db, dataDir: dataDir}, nil
}

func (d *DB) Close() error { return d.db.Close() }
func genID() string        { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func now() string          { return time.Now().UTC().Format(time.RFC3339) }

func (d *DB) filePath(bucketID, key string) string {
	return filepath.Join(d.dataDir, "files", bucketID, key)
}

// ── Buckets ──

func (d *DB) CreateBucket(b *Bucket) error {
	b.ID = genID()
	b.CreatedAt = now()
	pub := 0
	if b.Public {
		pub = 1
	}
	if err := os.MkdirAll(filepath.Join(d.dataDir, "files", b.ID), 0755); err != nil {
		return err
	}
	_, err := d.db.Exec(`INSERT INTO buckets (id,name,public,created_at) VALUES (?,?,?,?)`,
		b.ID, b.Name, pub, b.CreatedAt)
	return err
}

func (d *DB) hydrateBucket(b *Bucket) {
	d.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size),0) FROM objects WHERE bucket_id=?`, b.ID).Scan(&b.ObjectCount, &b.TotalSize)
}

func (d *DB) GetBucket(id string) *Bucket {
	var b Bucket
	var pub int
	if err := d.db.QueryRow(`SELECT id,name,public,created_at FROM buckets WHERE id=?`, id).Scan(&b.ID, &b.Name, &pub, &b.CreatedAt); err != nil {
		return nil
	}
	b.Public = pub == 1
	d.hydrateBucket(&b)
	return &b
}

func (d *DB) GetBucketByName(name string) *Bucket {
	var b Bucket
	var pub int
	if err := d.db.QueryRow(`SELECT id,name,public,created_at FROM buckets WHERE name=?`, name).Scan(&b.ID, &b.Name, &pub, &b.CreatedAt); err != nil {
		return nil
	}
	b.Public = pub == 1
	d.hydrateBucket(&b)
	return &b
}

func (d *DB) ListBuckets() []Bucket {
	rows, err := d.db.Query(`SELECT id,name,public,created_at FROM buckets ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Bucket
	for rows.Next() {
		var b Bucket
		var pub int
		rows.Scan(&b.ID, &b.Name, &pub, &b.CreatedAt)
		b.Public = pub == 1
		d.hydrateBucket(&b)
		out = append(out, b)
	}
	return out
}

func (d *DB) DeleteBucket(id string) error {
	// Delete all files
	objects := d.ListObjects(id, "")
	for _, obj := range objects {
		os.Remove(d.filePath(id, obj.Key))
	}
	os.RemoveAll(filepath.Join(d.dataDir, "files", id))
	d.db.Exec(`DELETE FROM objects WHERE bucket_id=?`, id)
	_, err := d.db.Exec(`DELETE FROM buckets WHERE id=?`, id)
	return err
}

// ── Objects ──

func (d *DB) PutObject(bucketID, key, contentType string, reader io.Reader) (*Object, error) {
	b := d.GetBucket(bucketID)
	if b == nil {
		return nil, fmt.Errorf("bucket not found")
	}

	// Ensure directory for the key
	fp := d.filePath(bucketID, key)
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return nil, err
	}

	// Write file
	f, err := os.Create(fp)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), reader)
	f.Close()
	if err != nil {
		os.Remove(fp)
		return nil, err
	}

	hash := hex.EncodeToString(h.Sum(nil))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upsert
	var existingID string
	err = d.db.QueryRow(`SELECT id FROM objects WHERE bucket_id=? AND key=?`, bucketID, key).Scan(&existingID)
	t := now()
	if err == sql.ErrNoRows {
		id := genID()
		d.db.Exec(`INSERT INTO objects (id,bucket_id,key,content_type,size,hash,created_at) VALUES (?,?,?,?,?,?,?)`,
			id, bucketID, key, contentType, size, hash, t)
		return &Object{ID: id, BucketID: bucketID, Key: key, ContentType: contentType, Size: size, Hash: hash, CreatedAt: t}, nil
	}
	d.db.Exec(`UPDATE objects SET content_type=?,size=?,hash=?,created_at=? WHERE id=?`, contentType, size, hash, t, existingID)
	return &Object{ID: existingID, BucketID: bucketID, Key: key, ContentType: contentType, Size: size, Hash: hash, CreatedAt: t}, nil
}

func (d *DB) GetObject(bucketID, key string) *Object {
	var o Object
	if err := d.db.QueryRow(`SELECT id,bucket_id,key,content_type,size,hash,created_at FROM objects WHERE bucket_id=? AND key=?`,
		bucketID, key).Scan(&o.ID, &o.BucketID, &o.Key, &o.ContentType, &o.Size, &o.Hash, &o.CreatedAt); err != nil {
		return nil
	}
	return &o
}

func (d *DB) GetObjectFile(bucketID, key string) (string, *Object) {
	obj := d.GetObject(bucketID, key)
	if obj == nil {
		return "", nil
	}
	return d.filePath(bucketID, key), obj
}

func (d *DB) ListObjects(bucketID, prefix string) []Object {
	q := `SELECT o.id,o.bucket_id,o.key,o.content_type,o.size,o.hash,o.created_at FROM objects o WHERE o.bucket_id=?`
	args := []any{bucketID}
	if prefix != "" {
		q += ` AND o.key LIKE ?`
		args = append(args, prefix+"%")
	}
	q += ` ORDER BY o.key`
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		var o Object
		rows.Scan(&o.ID, &o.BucketID, &o.Key, &o.ContentType, &o.Size, &o.Hash, &o.CreatedAt)
		out = append(out, o)
	}
	return out
}

func (d *DB) DeleteObject(bucketID, key string) error {
	os.Remove(d.filePath(bucketID, key))
	_, err := d.db.Exec(`DELETE FROM objects WHERE bucket_id=? AND key=?`, bucketID, key)
	return err
}

func (d *DB) IsBucketPublic(bucketID string) bool {
	var pub int
	d.db.QueryRow(`SELECT public FROM buckets WHERE id=?`, bucketID).Scan(&pub)
	return pub == 1
}

// ── Stats ──

type Stats struct {
	Buckets    int   `json:"buckets"`
	Objects    int   `json:"objects"`
	TotalBytes int64 `json:"total_bytes"`
}

func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow(`SELECT COUNT(*) FROM buckets`).Scan(&s.Buckets)
	d.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size),0) FROM objects`).Scan(&s.Objects, &s.TotalBytes)
	return s
}

func FormatSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}
