// Command server runs hushtongue. "server healthcheck" checks that a running
// server answers, for the Docker HEALTHCHECK.
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

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server"
	"github.com/wolf29f/hushtongue/internal/services/storage"
	"github.com/wolf29f/hushtongue/internal/services/storage/postgres"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		listen, err := config.LoadListen(nil)
		if err == nil {
			err = healthcheck(listen.Addr)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// run serves until SIGINT or SIGTERM, then lets the requests in progress
// finish.
func run(logger *slog.Logger) error {
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	store := postgres.New(db)
	if err := store.SetGM(ctx, cfg.GMUsername); err != nil {
		if errors.Is(err, storage.ErrNameTaken) {
			return fmt.Errorf("invalid configuration: HUSHTONGUE_GM_USERNAME %q is a player's name", cfg.GMUsername)
		}
		return fmt.Errorf("setting up the GM: %w", err)
	}

	app := server.New(cfg, store, logger)
	go app.PurgeSessions(ctx)

	srv := app.HTTPServer(cfg.Addr)
	served := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.Addr, "version", config.Version)
		served <- srv.ListenAndServe()
	}()

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}

	logger.Info("server shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
