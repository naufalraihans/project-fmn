package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fmn/server/internal/config"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/transport"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("konfigurasi tidak valid", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database tidak siap", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: transport.NewRouter(transport.Deps{
			Cfg:            cfg,
			Pool:           pool,
			InquiryLimiter: middleware.NewRateLimit(cfg.InquiryRateLimit, cfg.InquiryRateWindow),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		slog.Info("server mulai", "addr", cfg.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server berhenti tak terduga", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("mematikan server (menunggu request aktif selesai)")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("gagal shutdown rapi", "err", err)
	}
}
