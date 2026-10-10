package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	cacheMaxItems = 512
	cacheTTL      = 30 * time.Second
)

type cacheEntry struct {
	status  int
	headers http.Header
	body    []byte
	etag    string
}

// ResponseCache is an in-memory TTL LRU cache for GET responses.
type ResponseCache struct {
	lru *lru.LRU[string, *cacheEntry]
}

func newResponseCache() *ResponseCache {
	return &ResponseCache{
		lru: lru.NewLRU[string, *cacheEntry](cacheMaxItems, nil, cacheTTL),
	}
}

func (c *ResponseCache) get(key string) (*cacheEntry, bool) {
	return c.lru.Get(key)
}

func (c *ResponseCache) set(key string, entry *cacheEntry) {
	c.lru.Add(key, entry)
}

// computeETag returns a quoted ETag string computed from the body bytes.
func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf(`"%x"`, sum[:8])
}

// capturingWriter buffers the response so the cache middleware can inspect it
// before flushing to the real ResponseWriter.
type capturingWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	wrote  bool
}

func newCapturingWriter(w http.ResponseWriter) *capturingWriter {
	return &capturingWriter{ResponseWriter: w, status: http.StatusOK}
}

func (c *capturingWriter) WriteHeader(code int) {
	c.status = code
	c.wrote = true
}

func (c *capturingWriter) Write(b []byte) (int, error) {
	if !c.wrote {
		c.status = http.StatusOK
		c.wrote = true
	}
	return c.body.Write(b)
}

// flush writes the buffered response to the underlying ResponseWriter.
// Call only once; headers must already be set on c.ResponseWriter.Header().
func (c *capturingWriter) flush() {
	c.ResponseWriter.WriteHeader(c.status)
	_, _ = c.ResponseWriter.Write(c.body.Bytes())
}

// cacheMiddleware is a chi-compatible middleware that caches GET responses.
func (s *Server) cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		key := r.URL.RequestURI()

		if entry, ok := s.cache.get(key); ok {
			inm := r.Header.Get("If-None-Match")
			if inm != "" && inm == entry.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			for k, vs := range entry.headers {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.Header().Set("ETag", entry.etag)
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(entry.status)
			_, _ = w.Write(entry.body)
			return
		}

		cw := newCapturingWriter(w)
		next.ServeHTTP(cw, r)

		if cw.status == http.StatusOK {
			body := cw.body.Bytes()
			etag := computeETag(body)
			snapshot := make(http.Header, len(w.Header()))
			for k, vs := range w.Header() {
				snapshot[k] = append([]string(nil), vs...)
			}
			s.cache.set(key, &cacheEntry{
				status:  cw.status,
				headers: snapshot,
				body:    body,
				etag:    etag,
			})
			w.Header().Set("ETag", etag)
			w.Header().Set("X-Cache", "MISS")
		}
		cw.flush()
	})
}
