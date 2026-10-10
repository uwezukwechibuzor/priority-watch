// Package obs provides structured logging (slog) and an HTTP server that
// exposes /healthz and /metrics for any Priority Watch binary.
package obs

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server bundles a slog logger, a Prometheus registry, and an HTTP server
// that serves /healthz and /metrics on a single address.
type Server struct {
	log  *slog.Logger
	reg  *prometheus.Registry
	mux  *http.ServeMux
	addr string
}

// New creates a Server. addr is the TCP address to listen on (e.g. ":9091").
// level controls the minimum log level ("debug", "info", "warn", "error").
func New(addr, level string) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &Server{
		log:  newLogger(level),
		reg:  reg,
		mux:  mux,
		addr: addr,
	}
}

// Logger returns the structured JSON logger.
func (s *Server) Logger() *slog.Logger { return s.log }

// Registry returns the Prometheus registry so callers can register metrics.
func (s *Server) Registry() *prometheus.Registry { return s.reg }

// Serve starts the HTTP server and blocks until ctx is cancelled, then
// performs a graceful shutdown.  It returns the shutdown error, not the
// context error — a nil return means the server shut down cleanly.
func (s *Server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:        s.addr,
		Handler:     s.mux,
		ReadTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "obs server listening", "addr", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newLogger creates a JSON slog.Logger at the requested level.
func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
