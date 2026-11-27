package main

import (
	"context"
	"os"
	"time"

	"github.com/mahitss/evidence_architecture/backend-go/internal/config"
	"github.com/mahitss/evidence_architecture/backend-go/internal/queue"
	"github.com/mahitss/evidence_architecture/backend-go/internal/server"
	"github.com/mahitss/evidence_architecture/backend-go/internal/storage"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
	"github.com/rs/zerolog"
)

func main() {
	bootLogger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		bootLogger.Fatal().Err(err).Msg("failed to load configuration")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err == nil {
		bootLogger = bootLogger.Level(level)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		cancel()
		bootLogger.Fatal().Err(err).Msg("failed to initialize database store")
	}
	defer st.Close()

	objStorage, err := storage.NewLocalStorage(cfg.LocalStorageRoot)
	if err != nil {
		cancel()
		bootLogger.Fatal().Err(err).Msg("failed to initialize local object storage")
	}

	pub, err := queue.NewPublisher(ctx, cfg.RedisURL)
	cancel()
	if err != nil {
		bootLogger.Fatal().Err(err).Msg("failed to initialize redis streams publisher")
	}
	defer func() { _ = pub.Close() }()

	srv, err := server.NewWithDeps(cfg, st, objStorage, pub)
	if err != nil {
		bootLogger.Fatal().Err(err).Msg("failed to construct HTTP server")
	}

	bootLogger.Info().
		Str("listen_addr", cfg.ListenAddr).
		Str("log_level", cfg.LogLevel).
		Str("storage_root", cfg.LocalStorageRoot).
		Msg("starting backend-go api")

	if err := srv.Start(cfg.ListenAddr); err != nil {
		bootLogger.Fatal().Err(err).Msg("server exited with error")
	}

	bootLogger.Info().Msg("backend-go api shutdown complete")
}
