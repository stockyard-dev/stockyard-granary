package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/stockyard-dev/stockyard-granary/internal/store"
)

type Server struct {
	db     *store.DB
	mux    *http.ServeMux
	limits Limits
}

func New(db *store.DB, limits Limits) *Server {
	s := &Server{db: db, mux: http.NewServeMux(), limits: limits}

	s.mux.HandleFunc("GET /api/buckets", s.listBuckets)
	s.mux.HandleFunc("POST /api/buckets", s.createBucket)
	s.mux.HandleFunc("GET /api/buckets/{id}", s.getBucket)
	s.mux.HandleFunc("DELETE /api/buckets/{id}", s.deleteBucket)

	s.mux.HandleFunc("GET /api/buckets/{id}/objects", s.listObjects)
	s.mux.HandleFunc("PUT /api/buckets/{bid}/objects/{key...}", s.putObject)
	s.mux.HandleFunc("GET /api/buckets/{bid}/objects/{key...}", s.getObject)
	s.mux.HandleFunc("DELETE /api/buckets/{bid}/objects/{key...}", s.deleteObject)

	s.mux.HandleFunc("POST /api/upload/{bid}", s.upload)

	s.mux.HandleFunc("GET /api/stats", s.stats)
	s.mux.HandleFunc("GET /api/health", s.health)

	s.mux.HandleFunc("GET /ui", s.dashboard)
	s.mux.HandleFunc("GET /ui/", s.dashboard)
	s.mux.HandleFunc("GET /", s.root)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Public file serving: /f/{bucket-name}/{key}
	if strings.HasPrefix(r.URL.Path, "/f/") {
		s.servePublic(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (s *Server) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", http.StatusFound)
}

// ── Buckets ──

func (s *Server) listBuckets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"buckets": orEmpty(s.db.ListBuckets())})
}

func (s *Server) createBucket(w http.ResponseWriter, r *http.Request) {
	var b store.Bucket
	json.NewDecoder(r.Body).Decode(&b)
	if b.Name == "" {
		writeErr(w, 400, "name required")
		return
	}
	if err := s.db.CreateBucket(&b); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, s.db.GetBucket(b.ID))
}

func (s *Server) getBucket(w http.ResponseWriter, r *http.Request) {
	b := s.db.GetBucket(r.PathValue("id"))
	if b == nil {
		writeErr(w, 404, "not found")
		return
	}
	writeJSON(w, 200, b)
}

func (s *Server) deleteBucket(w http.ResponseWriter, r *http.Request) {
	s.db.DeleteBucket(r.PathValue("id"))
	writeJSON(w, 200, map[string]string{"deleted": "ok"})
}

// ── Objects ──

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	writeJSON(w, 200, map[string]any{"objects": orEmpty(s.db.ListObjects(r.PathValue("id"), prefix))})
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request) {
	bucketID := r.PathValue("bid")
	key := r.PathValue("key")
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	obj, err := s.db.PutObject(bucketID, key, ct, r.Body)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, obj)
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	bucketID := r.PathValue("bid")
	key := r.PathValue("key")
	fp, obj := s.db.GetObjectFile(bucketID, key)
	if obj == nil {
		writeErr(w, 404, "not found")
		return
	}
	w.Header().Set("Content-Type", obj.ContentType)
	http.ServeFile(w, r, fp)
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request) {
	s.db.DeleteObject(r.PathValue("bid"), r.PathValue("key"))
	writeJSON(w, 200, map[string]string{"deleted": "ok"})
}

// ── Multipart upload ──

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	bucketID := r.PathValue("bid")
	if s.db.GetBucket(bucketID) == nil {
		writeErr(w, 404, "bucket not found")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "file field required")
		return
	}
	defer file.Close()

	key := r.FormValue("key")
	if key == "" {
		key = header.Filename
	}
	ct := header.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}

	obj, err := s.db.PutObject(bucketID, key, ct, file)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, obj)
}

// ── Public file serving ──

func (s *Server) servePublic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/f/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	bucketName := parts[0]
	key := parts[1]

	bucket := s.db.GetBucketByName(bucketName)
	if bucket == nil {
		http.NotFound(w, r)
		return
	}
	if !bucket.Public {
		writeErr(w, 403, "bucket is not public")
		return
	}

	fp, obj := s.db.GetObjectFile(bucket.ID, key)
	if obj == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, fp)
}

// ── Meta ──

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	st := s.db.Stats()
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"service": "granary",
		"objects": st.Objects,
		"size":    store.FormatSize(st.TotalBytes),
	})
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
