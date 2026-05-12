package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	httpapi "github.com/6ermvH/log-parser/internal/api/v1/http"
	"github.com/6ermvH/log-parser/internal/config"
	"github.com/6ermvH/log-parser/internal/logger"
	"github.com/6ermvH/log-parser/internal/parser"
	"github.com/6ermvH/log-parser/internal/reaper"
	"github.com/6ermvH/log-parser/internal/service"
	"github.com/6ermvH/log-parser/internal/storage/migrate"
	"github.com/6ermvH/log-parser/internal/storage/postgres"
	"github.com/6ermvH/log-parser/migrations"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	bootstrapLog := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(); err != nil {
		bootstrapLog.Error("startup failed", "err", err)
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

	repo := postgres.NewRepository(pool)
	parserSvc := parser.New()
	parseService := service.NewParseService(parserSvc, repo, log)
	queryService := service.NewQueryService(repo)
	reaperInst := reaper.New(repo, cfg.Reaper.Tick, cfg.Reaper.Timeout, log)

	reaperCtx, stopReaper := context.WithCancel(context.Background())

	var reaperWG sync.WaitGroup
	reaperWG.Add(1)

	defer reaperWG.Wait()
	defer stopReaper()

	go func() {
		defer reaperWG.Done()

		reaperInst.Run(reaperCtx)
	}()

	handler := httpapi.NewRouter(httpapi.Dependencies{
		ParseService: parseService,
		QueryService: queryService,
		Pool:         pool,
		Logger:       log,
		DataDir:      cfg.DataDir,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
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
