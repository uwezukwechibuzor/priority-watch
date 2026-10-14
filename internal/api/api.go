// Package api implements the Priority Watch REST API.
// Routes mount at /api/v1/*; OpenAPI 3.1 at /openapi.json; Scalar UI at /docs.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/uwezukwechibuzor/priority-watch/internal/config"
)

// Server is the pw-api HTTP server.
type Server struct {
	cfg    *config.Config
	router *chi.Mux
	api    huma.API
	store  *Store
	cache  *ResponseCache
	obs    *prometheus.Registry
	log    *slog.Logger
	btr    *BacktestRunner

	reqDuration *prometheus.HistogramVec

	// headFunc returns the live chain tip block number. Optional — if nil,
	// max_block_number in KPI responses falls back to 0.
	headFunc func(context.Context) (uint64, error)

	// rate limiter references stored so tests can swap them
	rateLimiterAPI      func(http.Handler) http.Handler
	rateLimiterDatasets func(http.Handler) http.Handler
}

// WithHeadFunc sets the function used to fetch the live chain tip block number.
func (s *Server) WithHeadFunc(f func(context.Context) (uint64, error)) *Server {
	s.headFunc = f
	return s
}

// New creates a Server with the given config and read-only ClickHouse connection.
func New(cfg *config.Config, conn driver.Conn, log *slog.Logger) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	reqDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pw",
		Subsystem: "api",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency by handler path.",
		Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"handler", "method", "status"})
	reg.MustRegister(reqDuration)

	st := newStore(conn)
	s := &Server{
		cfg:                 cfg,
		router:              chi.NewRouter(),
		store:               st,
		cache:               newResponseCache(),
		obs:                 reg,
		log:                 log,
		btr:                 newBacktestRunner(st, 4, log),
		reqDuration:         reqDuration,
		rateLimiterAPI:      httprate.LimitByIP(60, time.Minute), //nolint:staticcheck
		rateLimiterDatasets: httprate.LimitByIP(10, time.Minute), //nolint:staticcheck
	}

	// Build chi middleware stack (recovery, CORS, request ID, logging).
	s.buildMiddlewareStack()

	// Huma API — OpenAPI 3.1 spec generated from handler types.
	humaConf := huma.DefaultConfig("Priority Watch API", "1.0.0")
	humaConf.Info.Description = "Analytics API for Arbitrum One Priority Gas Auction (PGA) ordering metrics."
	humaConf.Servers = []*huma.Server{{URL: "/", Description: "This server"}}
	humaConf.DocsPath = "" // we serve Scalar ourselves below
	s.api = humachi.New(s.router, humaConf)

	// Mount all route groups.
	s.mountRoutes()

	return s
}

// mountRoutes registers every endpoint on the chi router + huma API.
func (s *Server) mountRoutes() {
	r := s.router

	// Infra endpoints (not under /api/v1, not cached, not rate-limited).
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
	})
	r.Handle("/metrics", promhttp.HandlerFor(s.obs, promhttp.HandlerOpts{}))

	// Scalar UI at /docs — loads the OpenAPI spec from /openapi.json.
	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, scalarHTML)
	})

	// /api/v1/* with rate-limiting and ETag response cache.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(s.rateLimiterAPI)
		r.Use(s.cacheMiddleware)

		// Register typed huma handlers (all mounted on s.api which uses r).
		registerBlockHandlers(s.api, s.store, func(ctx context.Context) (uint64, error) {
				if s.headFunc == nil {
					return 0, nil
				}
				return s.headFunc(ctx)
			})
		registerHealthHandlers(s.api, s.store)
		registerTreasuryHandlers(s.api, s.store)
		registerCompareHandlers(s.api, s.store)
		registerOpportunityHandlers(s.api, s.store)
		registerExecutionHandlers(s.api, s.store)
		registerFeeHandlers(s.api, s.store)
		registerEventHandlers(s.api, s.store)
		registerSearchHandlers(s.api, s.store)
		registerPricingHandlers(s.api, s.store)
		registerModelEvalHandlers(s.api, s.store)
		registerBacktestHandlers(s.api, s.btr)
		registerContractHandlers(s.api, s.store)
	})

	// Datasets endpoint: separate (lower) rate limit, no cache (streaming).
	s.registerDatasetRoutes()
}

// Handler returns the underlying http.Handler for use in tests or embedding.
func (s *Server) Handler() http.Handler { return s.router }

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.APIAddr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.log.InfoContext(ctx, "api listening", "addr", s.cfg.APIAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

// scalarHTML is the Scalar API reference UI that loads /openapi.json.
const scalarHTML = `<!doctype html>
<html>
<head>
  <title>Priority Watch API</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`
