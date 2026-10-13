package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// buildMiddlewareStack attaches all chi middleware to s.router.
// Called once from New() before any routes are registered.
func (s *Server) buildMiddlewareStack() {
	r := s.router

	// Recovery — must be outermost to catch panics from all middleware below.
	r.Use(middleware.Recoverer)

	// Request IDs for log correlation.
	r.Use(middleware.RequestID)

	// CORS.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{s.cfg.APICORSOrigin},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "If-None-Match"},
		ExposedHeaders:   []string{"ETag", "X-Cache", "X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Structured request logging.
	r.Use(s.requestLogger)

	// Cache + ETag — applied to the whole /api/v1 subtree in mountAPI().
}

// requestLogger is a chi middleware that emits one slog line per request
// and records pw_api_request_duration_seconds for Prometheus.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			elapsed := time.Since(start)
			status := ww.Status()
			s.log.InfoContext(r.Context(), "http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", ww.BytesWritten(),
				"duration_ms", elapsed.Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
			if s.reqDuration != nil {
				s.reqDuration.WithLabelValues(
					r.URL.Path,
					r.Method,
					fmt.Sprintf("%d", status),
				).Observe(elapsed.Seconds())
			}
		}()
		next.ServeHTTP(ww, r)
	})
}
