package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"quality_control/internal/auth"
	"quality_control/internal/config"
	"quality_control/internal/db"
	"quality_control/internal/httpapi"
	"quality_control/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))
	if cfg.WarnWeakSecret {
		slog.Warn("QC_JWT_SECRET still looks like the example. Replace it before exposing this server.")
	}
	if cfg.WarnWeakAdmin {
		slog.Warn("QC_ADMIN_PASSWORD still looks like the example. Change it after the first sign-in.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migrateCtx, cancelMigrate := context.WithTimeout(ctx, 2*time.Minute)
	err = db.Migrate(migrateCtx, cfg.DatabaseURL)
	cancelMigrate()
	if err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	st := store.New(pool)
	if err := st.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminEmail, cfg.AdminFullName, cfg.AdminPassword); err != nil {
		slog.Error("administrator", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "attachments"), 0o750); err != nil {
		slog.Error("data directory", "err", err)
		os.Exit(1)
	}

	api := httpapi.New(cfg, st, auth.NewManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL))
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "err", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("stopped")
}
