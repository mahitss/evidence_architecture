package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/auth"
	"github.com/mahitss/evidence_architecture/backend-go/internal/config"
	"github.com/mahitss/evidence_architecture/backend-go/internal/handlers"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/search"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type redisClientProvider interface {
	Client() *redis.Client
}

// Server encapsulates the HTTP router, dependencies, and lifecycle management.
type Server struct {
	cfg         config.Config
	store       *store.Store
	router      chi.Router
	logger      zerolog.Logger
	authHandler *handlers.AuthHandler
}

// New constructs a Server using LocalStorage (from cfg.LocalStorageRoot) and a nil queue
// publisher if none is supplied explicitly. Use NewWithDeps or NewWithAllDeps when wiring Redis/Meilisearch.
func New(cfg config.Config, st *store.Store) (*Server, error) {
	ls, err := storage.NewLocalStorage(cfg.LocalStorageRoot)
	if err != nil {
		return nil, fmt.Errorf("init local storage: %w", err)
	}
	return NewWithDeps(cfg, st, ls, nil)
}

// NewWithDeps constructs a Server with explicit Storage and EvidencePublisher dependencies.
// If publisher exposes a *redis.Client, RedisIdempotencyStore and RedisRateLimiter are wired automatically.
func NewWithDeps(
	cfg config.Config,
	st *store.Store,
	objStorage storage.Storage,
	publisher queue.EvidencePublisher,
) (*Server, error) {
	var idemStore middleware.IdempotencyStore
	var rateLimiter middleware.RateLimiter
	if provider, ok := publisher.(redisClientProvider); ok && provider != nil {
		if rc := provider.Client(); rc != nil {
			idemStore = middleware.NewRedisIdempotencyStore(rc)
			rateLimiter = middleware.NewRedisRateLimiter(rc)
		}
	}
	return NewWithAllDeps(cfg, st, objStorage, publisher, idemStore, nil, rateLimiter)
}

// NewWithIdempotency constructs a Server with explicit Storage, EvidencePublisher, and IdempotencyStore dependencies.
func NewWithIdempotency(
	cfg config.Config,
	st *store.Store,
	objStorage storage.Storage,
	publisher queue.EvidencePublisher,
	idemStore middleware.IdempotencyStore,
) (*Server, error) {
	var rateLimiter middleware.RateLimiter
	if provider, ok := publisher.(redisClientProvider); ok && provider != nil {
		if rc := provider.Client(); rc != nil {
			rateLimiter = middleware.NewRedisRateLimiter(rc)
		}
	}
	return NewWithAllDeps(cfg, st, objStorage, publisher, idemStore, nil, rateLimiter)
}

// NewWithAllDeps constructs a Server with all runtime dependencies including Meilisearch indexing
// and rate limiting.
// Middleware order:
//   - Global: requestid -> logging -> recoverer -> errorenvelope
//   - Reader routes (/api/v1/cases*, /api/v1/evidence*, /api/v1/search, /api/v1/corrections):
//     Geoblock FIRST (EU/EEA/UK -> HTTP 451 before touching rate limits) -> RateLimit -> (Idempotency on POST /corrections)
//   - Admin routes (/api/v1/admin/*) and /health: NO Geoblock (admins operate from any region)
func NewWithAllDeps(
	cfg config.Config,
	st *store.Store,
	objStorage storage.Storage,
	publisher queue.EvidencePublisher,
	idemStore middleware.IdempotencyStore,
	searchIndexer search.Indexer,
	rateLimiter middleware.RateLimiter,
) (*Server, error) {
	if st == nil {
		return nil, errors.New("store must not be nil")
	}
	if objStorage == nil {
		ls, err := storage.NewLocalStorage(cfg.LocalStorageRoot)
		if err != nil {
			return nil, fmt.Errorf("init default local storage: %w", err)
		}
		objStorage = ls
	}
	if rateLimiter == nil {
		rateLimiter = middleware.NewMemoryRateLimiter()
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stdout).Level(level).With().Timestamp().Logger()

	jwtManager, err := auth.NewJWTManager(cfg.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("init jwt manager: %w", err)
	}

	healthHandler := handlers.NewHealthHandler(st, cfg.Version)
	authHandler := handlers.NewAuthHandler(
		st.AdminUsers,
		jwtManager,
		cfg.JWTTTL,
		cfg.LoginRateLimitPerMin,
		logger,
	)
	adminCasesHandler := handlers.NewAdminCasesHandler(st.Cases, st.LegalReviews, st.Evidence, objStorage)
	adminCaseWriteHandler := handlers.NewAdminCaseWriteHandler(st.Cases).WithIndexer(searchIndexer)
	adminEvidenceWriteHandler := handlers.NewAdminEvidenceWriteHandler(st.Evidence, objStorage, publisher, logger).WithIndexer(searchIndexer)
	adminPublishHandler := handlers.NewAdminPublishHandler(st.LegalReviews, st.Cases).WithIndexer(searchIndexer)
	readerHandler := handlers.NewReaderHandler(st.Cases, st.Evidence, st.Corrections, objStorage, searchIndexer)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(logger))
	r.Use(chimw.Recoverer)
	r.Use(middleware.ErrorEnvelope)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		middleware.WriteError(w, req, http.StatusNotFound, "NOT_FOUND", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		middleware.WriteError(w, req, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	// Public health probe (no auth, no geoblock)
	r.Get("/health", healthHandler.ServeHTTP)

	// Public anonymous reader API routes (NO auth per ADR-007; Geoblock FIRST, then per-route RateLimit)
	r.Route("/api/v1", func(pub chi.Router) {
		pub.Group(func(reader chi.Router) {
			reader.Use(middleware.Geoblock)

			reader.With(middleware.RateLimit(rateLimiter, middleware.RateLimitConfig{
				Bucket: "cases_list",
				Limit:  60,
				Window: 60,
			})).Get("/cases", readerHandler.ListCases)

			reader.With(middleware.RateLimit(rateLimiter, middleware.RateLimitConfig{
				Bucket: "case_detail",
				Limit:  60,
				Window: 60,
			})).Get("/cases/{slug}", readerHandler.GetCaseBySlug)

			reader.With(middleware.RateLimit(rateLimiter, middleware.RateLimitConfig{
				Bucket: "evidence_detail",
				Limit:  120,
				Window: 60,
			})).Get("/evidence/{id}", readerHandler.GetEvidenceByID)

			reader.With(middleware.RateLimit(rateLimiter, middleware.RateLimitConfig{
				Bucket: "search",
				Limit:  30,
				Window: 60,
			})).Get("/search", readerHandler.Search)

			reader.With(
				middleware.RateLimit(rateLimiter, middleware.RateLimitConfig{
					Bucket:     "corrections",
					Limit:      3,
					Window:     3600,
					FailClosed: true,
				}),
				middleware.Idempotency(idemStore),
			).Post("/corrections", readerHandler.SubmitCorrection)
		})

		// Admin API routes (NO Geoblock — authorized admins may operate from any region)
		pub.Route("/admin", func(ar chi.Router) {
			// Unauthenticated login endpoint with in-memory IP rate limit (not idempotent)
			ar.Post("/login", authHandler.Login)

			// JWT-protected admin routes
			ar.Group(func(protected chi.Router) {
				protected.Use(middleware.RequireAdminJWT(jwtManager))

				// Read-only admin routes (no Idempotency-Key required)
				protected.Get("/cases/{id}", adminCasesHandler.GetByID)

				// Mutating admin write routes (Idempotency-Key middleware enforced on all 7 endpoints)
				protected.Group(func(idempotent chi.Router) {
					idempotent.Use(middleware.Idempotency(idemStore))

					// Cases
					idempotent.Post("/cases", adminCaseWriteHandler.Create)
					idempotent.Put("/cases/{id}", adminCaseWriteHandler.Update)
					idempotent.Post("/cases/{id}/retract", adminCaseWriteHandler.Retract)

					// Evidence
					idempotent.Post("/cases/{id}/evidence", adminEvidenceWriteHandler.Upload)
					idempotent.Put("/evidence/{id}", adminEvidenceWriteHandler.UpdateMetadata)

					// Legal review & publish gate
					idempotent.Post("/cases/{id}/legal-review", adminPublishHandler.RecordLegalReview)
					idempotent.Post("/cases/{id}/publish", adminPublishHandler.Publish)
				})
			})
		})
	})

	return &Server{
		cfg:         cfg,
		store:       st,
		router:      r,
		logger:      logger,
		authHandler: authHandler,
	}, nil
}

// Handler returns the underlying http.Handler for httptest or embedding.
func (s *Server) Handler() http.Handler {
	return s.router
}

// AuthHandler returns the login handler (used in tests to tune failure delay cleanly).
func (s *Server) AuthHandler() *handlers.AuthHandler {
	return s.authHandler
}

// Start runs the HTTP server on addr and performs graceful shutdown on SIGINT or SIGTERM.
func (s *Server) Start(addr string) error {
	if addr == "" {
		addr = s.cfg.ListenAddr
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("addr", addr).Str("version", s.cfg.Version).Msg("http_server_listening")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	case sig := <-sigCh:
		s.logger.Info().Str("signal", sig.String()).Msg("http_server_shutting_down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		s.logger.Info().Msg("http_server_stopped")
		return nil
	}
}
