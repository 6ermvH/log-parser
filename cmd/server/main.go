package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/6ermvH/log-parser/internal/config"
	"github.com/6ermvH/log-parser/internal/logger"
	"github.com/6ermvH/log-parser/internal/storage/migrate"
	"github.com/6ermvH/log-parser/internal/storage/postgres"
	"github.com/6ermvH/log-parser/migrations"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
	healthPingTimeout = 2 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("starting", "port", cfg.Port, "data_dir", cfg.DataDir)

	if mErr := migrate.Run(migrations.FS, cfg.DatabaseURL); mErr != nil {
		return fmt.Errorf("migrations: %w", mErr)
	}

	log.Info("migrations applied")

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()

	log.Info("db connected")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(pool, log))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Info("listening", "addr", srv.Addr)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info("shutdown signal", "signal", sig.String())
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	log.Info("stopped")

	return nil
}

func healthHandler(pool *pgxpool.Pool, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")

		if err := pool.Ping(ctx); err != nil {
			log.Warn("health db ping failed", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)

			if _, werr := w.Write([]byte(`{"status":"unhealthy"}`)); werr != nil {
				log.Error("write health response", "err", werr)
			}

			return
		}

		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Error("write health response", "err", err)
		}
	}
}
