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
	"github.com/mahitss/evidence_architecture/backend-go/internal/server/middleware"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
)

// Server encapsulates the HTTP router, dependencies, and lifecycle management.
type Server struct {
	cfg         config.Config
	store       *store.Store
	router      chi.Router
	logger      zerolog.Logger
	authHandler *handlers.AuthHandler
}

// New constructs a Server using LocalStorage (from cfg.LocalStorageRoot) and a nil queue
// publisher if none is supplied explicitly. Use NewWithDeps when wiring Redis Streams publishing.
func New(cfg config.Config, st *store.Store) (*Server, error) {
	ls, err := storage.NewLocalStorage(cfg.LocalStorageRoot)
	if err != nil {
		return nil, fmt.Errorf("init local storage: %w", err)
	}
	return NewWithDeps(cfg, st, ls, nil)
}

// NewWithDeps constructs a Server with explicit Storage and EvidencePublisher dependencies.
// Middleware order: requestid -> logging -> recoverer -> errorenvelope -> router
// (JWT strictly on protected /api/v1/admin/* endpoints).
func NewWithDeps(
	cfg config.Config,
	st *store.Store,
	objStorage storage.Storage,
	publisher queue.EvidencePublisher,
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
	adminCaseWriteHandler := handlers.NewAdminCaseWriteHandler(st.Cases)
	adminEvidenceWriteHandler := handlers.NewAdminEvidenceWriteHandler(st.Evidence, objStorage, publisher, logger)
	adminPublishHandler := handlers.NewAdminPublishHandler(st.LegalReviews, st.Cases)

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

	// Public health probe (no auth)
	r.Get("/health", healthHandler.ServeHTTP)

	// Admin API routes
	r.Route("/api/v1/admin", func(ar chi.Router) {
		// Unauthenticated login endpoint with in-memory IP rate limit
		ar.Post("/login", authHandler.Login)

		// JWT-protected admin read & write routes
		ar.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAdminJWT(jwtManager))

			// Cases
			protected.Post("/cases", adminCaseWriteHandler.Create)
			protected.Get("/cases/{id}", adminCasesHandler.GetByID)
			protected.Put("/cases/{id}", adminCaseWriteHandler.Update)
			protected.Post("/cases/{id}/retract", adminCaseWriteHandler.Retract)

			// Evidence
			protected.Post("/cases/{id}/evidence", adminEvidenceWriteHandler.Upload)
			protected.Put("/evidence/{id}", adminEvidenceWriteHandler.UpdateMetadata)

			// Legal review & publish gate
			protected.Post("/cases/{id}/legal-review", adminPublishHandler.RecordLegalReview)
			protected.Post("/cases/{id}/publish", adminPublishHandler.Publish)
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
